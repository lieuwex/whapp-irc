package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"regexp"
	"strings"
	"time"
	"whapp-irc/database"
	"whapp-irc/whapp"

	"github.com/avast/retry-go"
	"github.com/mitchellh/mapstructure"
	qrcode "github.com/skip2/go-qrcode"
	irc "gopkg.in/sorcix/irc.v2"
)

func logMessage(time time.Time, from, to, message string) {
	timeStr := time.Format("2006-01-02 15:04:05")
	log.Printf("(%s) %s->%s: %s", timeStr, from, to, message)
}

var replyRegex = regexp.MustCompile(`^!(\d+)\s+(.+)$`)

type Connection struct {
	Chats []*Chat

	nickname string
	me       whapp.Me

	startedNegotiation         bool
	negotiationFinished        bool
	negotiationFinishedChannel chan bool
	caps                       []string

	bridge *Bridge
	socket *net.TCPConn

	welcomed  bool
	welcomeCh chan bool

	localStorage map[string]string

	timestampMap *TimestampMap
}

func MakeConnection() (*Connection, error) {
	return &Connection{
		bridge: MakeBridge(),

		welcomeCh:                  make(chan bool),
		negotiationFinishedChannel: make(chan bool),

		timestampMap: MakeTimestampMap(),
	}, nil
}

func (conn *Connection) BindSocket(socket *net.TCPConn) error {
	defer socket.Close()
	defer conn.bridge.Stop()

	conn.socket = socket
	write := conn.writeIRCNow
	status := conn.status

	ctx, cancel := context.WithCancel(context.Background())

	// listen for and parse messages.
	// we want to do this outside the next irc message handle loop, so we can
	// reply to PINGs but not handle stuff like JOINs yet.
	ircCh := make(chan *irc.Message)
	go func() {
		defer close(ircCh)

		decoder := irc.NewDecoder(bufio.NewReader(socket))
		for {
			msg, err := decoder.Decode()
			if err != nil {
				return
			}

			if msg.Command == "PING" {
				write(":whapp-irc PONG whapp-irc :" + msg.Params[0])
				continue
			}

			ircCh <- msg
		}
	}()

	welcome := func() (setup bool, err error) {
		if conn.welcomed || conn.nickname == "" {
			return false, nil
		}

		conn.writeIRCNow(fmt.Sprintf(":whapp-irc 001 %s Welcome to whapp-irc, %s.", conn.nickname, conn.nickname))
		conn.writeIRCNow(fmt.Sprintf(":whapp-irc 002 %s Enjoy the ride.", conn.nickname))

		conn.welcomed = true

		err = retry.Do(func() error {
			conn.bridge.Stop()
			err := conn.setup()
			if err != nil {
				fmt.Printf("err while setting up: %s\n", err.Error())
			}
			return err
		}, retry.Attempts(5), retry.Delay(time.Second))
		if err != nil {
			return false, err
		}

		close(conn.welcomeCh)
		return true, nil
	}

	go func() {
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return

			case msg, ok := <-ircCh:
				if !ok {
					log.Println("error while listening for IRC messages")
					return
				}

				if msg.Command == "NICK" {
					conn.nickname = msg.Params[0]
					if _, err := welcome(); err != nil {
						status("giving up trying to setup whapp bridge: " + err.Error())
						return
					}
					continue
				}

				conn.handleIRCCommand(msg)
			}
		}
	}()

	<-conn.welcomeCh
	if conn.startedNegotiation {
		<-conn.negotiationFinishedChannel
	}

	empty := conn.timestampMap.Length() == 0
	for _, c := range conn.Chats {
		prevTimestamp, found := conn.timestampMap.Get(c.ID)

		if empty || !conn.HasCapability("whapp-irc/replay") {
			conn.timestampMap.Set(c.ID, c.rawChat.Timestamp)
			go conn.saveDatabaseEntry()
			continue
		} else if c.rawChat.Timestamp <= prevTimestamp {
			continue
		}

		if !found {
			// fetch all older messages
			prevTimestamp = math.MinInt64
		}

		messages, err := c.rawChat.GetMessagesFromChatTillDate(
			conn.bridge.ctx,
			conn.bridge.WI,
			prevTimestamp,
		)
		if err != nil {
			fmt.Printf("error while loading earlier messages: %s\n", err.Error())
			continue
		}

		for _, msg := range messages {
			if msg.Timestamp <= prevTimestamp {
				continue
			}

			if err := conn.handleWhappMessage(msg); err != nil {
				fmt.Printf("error handling older whapp message: %s\n", err.Error())
				continue
			}
		}
	}

	go func() {
		defer cancel()

		resCh, errCh := conn.bridge.WI.ListenLoggedIn(conn.bridge.ctx, time.Second)

		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				fmt.Printf("error while listening for whatsapp loggedin state: %s\n", err.Error())
				return

			case res := <-resCh:
				if res {
					continue
				}

				fmt.Println("logged out of whatsapp!")

				return
			}
		}
	}()

	go func() {
		defer cancel()

		messageCh, errCh := conn.bridge.WI.ListenForMessages(
			conn.bridge.ctx,
			500*time.Millisecond,
		)

		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				fmt.Printf("error while listening for whatsapp messages: %s\n", err.Error())
				return

			case msg := <-messageCh:
				if err := conn.handleWhappMessage(msg); err != nil {
					fmt.Printf("error handling new whapp message: %s\n", err.Error())
					continue
				}
			}

		}
	}()

	<-ctx.Done()
	log.Printf("connection ended: %s\n", ctx.Err())
	return nil
}

func (conn *Connection) joinChat(chat *Chat) error {
	if chat == nil {
		return fmt.Errorf("chat is nil")
	} else if !chat.IsGroupChat {
		return fmt.Errorf("not a group chat")
	} else if chat.Joined {
		return nil
	}

	identifier := chat.Identifier()

	conn.writeIRCNow(fmt.Sprintf(":%s JOIN %s", conn.nickname, identifier))
	conn.writeIRCNow(fmt.Sprintf(":whapp-irc 332 %s %s :%s", conn.nickname, identifier, chat.Name))

	names := make([]string, 0)
	for _, participant := range chat.Participants {
		if participant.Contact.IsMe {
			if participant.IsAdmin {
				conn.writeIRCNow(fmt.Sprintf(":whapp-irc MODE %s +o %s", identifier, conn.nickname))
			}
			continue
		}

		prefix := ""
		if participant.IsAdmin {
			prefix = "@"
		}

		names = append(names, prefix+participant.SafeName())
	}

	conn.writeIRCNow(fmt.Sprintf(":whapp-irc 353 %s @ %s :%s", conn.nickname, identifier, strings.Join(names, " ")))
	conn.writeIRCNow(fmt.Sprintf(":whapp-irc 366 %s %s :End of /NAMES list.", conn.nickname, identifier))

	chat.Joined = true
	return nil
}

func (conn *Connection) GetChatByID(ID string) *Chat {
	for _, c := range conn.Chats {
		if c.ID == ID {
			return c
		}
	}
	return nil
}

func (conn *Connection) GetChatByIdentifier(identifier string) *Chat {
	identifier = strings.ToLower(identifier)

	for _, c := range conn.Chats {
		if strings.ToLower(c.Identifier()) == identifier {
			return c
		}
	}
	return nil
}

func (conn *Connection) addChat(chat whapp.Chat) (*Chat, error) {
	participants, err := chat.Participants(conn.bridge.ctx, conn.bridge.WI)
	if err != nil {
		return nil, err
	}

	converted := make([]Participant, len(participants))
	for i, p := range participants {
		converted[i] = Participant(p)
	}

	res := &Chat{
		ID:   chat.ID,
		Name: chat.Title(),

		IsGroupChat:  chat.IsGroupChat,
		Participants: converted,

		Joined:     false,
		MessageIDs: make([]string, 0),

		rawChat: chat,
	}

	if chat.IsGroupChat {
		fmt.Printf("%-30s %3d participants\n", res.Identifier(), len(res.Participants))
	} else {
		fmt.Println(res.Identifier())
	}

	for i, c := range conn.Chats {
		if c.ID == chat.ID {
			conn.Chats[i] = res
			return res, nil
		}
	}
	conn.Chats = append(conn.Chats, res)
	return res, nil
}

// TODO: check if already setup
func (conn *Connection) setup() error {
	_, err := conn.bridge.Start()
	if err != nil {
		return err
	}

	obj, found, err := userDb.GetItem(conn.nickname)
	if err != nil {
		return err
	} else if found {
		var user database.User
		if err := mapstructure.Decode(obj, &user); err != nil {
			panic(err)
		}

		conn.timestampMap.Swap(user.LastReceivedReceipts)

		err := conn.bridge.WI.SetLocalStorage(conn.bridge.ctx, user.LocalStorage)
		if err != nil {
			fmt.Printf("error while setting local storage: %s\n", err.Error())
		}
	}

	state, err := conn.bridge.WI.Open(conn.bridge.ctx)
	if err != nil {
		return err
	}

	var qrFile *File
	if state == whapp.Loggedout {
		code, err := conn.bridge.WI.GetLoginCode(conn.bridge.ctx)
		if err != nil {
			return fmt.Errorf("Error while retrieving login code: %s", err.Error())
		}

		bytes, err := qrcode.Encode(code, qrcode.High, 512)
		if err != nil {
			return err
		}

		qrFile, err = fs.AddBlob("qr-"+strTimestamp(), "png", bytes)
		if err != nil {
			return err
		}

		conn.status("Scan this QR code: " + qrFile.URL)
	}

	if err := conn.bridge.WI.WaitLogin(conn.bridge.ctx); err != nil {
		return err
	}
	conn.status("logged in")

	conn.localStorage, err = conn.bridge.WI.GetLocalStorage(conn.bridge.ctx)
	if err != nil {
		fmt.Printf("error while getting local storage: %s\n", err.Error())
	} else {
		if err := conn.saveDatabaseEntry(); err != nil {
			return err
		}
	}

	if qrFile != nil {
		if err = fs.RemoveFile(qrFile); err != nil {
			fmt.Printf("error while removing QR code: %s\n", err.Error())
		}
	}

	conn.me, err = conn.bridge.WI.GetMe(conn.bridge.ctx)
	if err != nil {
		return err
	}

	chats, err := conn.bridge.WI.GetAllChats(conn.bridge.ctx)
	if err != nil {
		return err
	}
	for _, chat := range chats {
		if _, err := conn.addChat(chat); err != nil {
			return err
		}
	}

	return nil
}

func (conn *Connection) getPresenceByUserID(userID string) (presence whapp.Presence, found bool, err error) {
	for _, c := range conn.Chats {
		if c.ID == userID {
			presence, err := c.rawChat.GetPresence(conn.bridge.ctx, conn.bridge.WI)
			return presence, true, err
		}
	}

	return whapp.Presence{}, false, nil
}

func (conn *Connection) saveDatabaseEntry() error {
	err := userDb.SaveItem(conn.nickname, database.User{
		Nickname:             conn.nickname,
		LocalStorage:         conn.localStorage,
		LastReceivedReceipts: conn.timestampMap.GetCopy(),
	})
	if err != nil {
		log.Printf("error while updating user entry: %s\n", err.Error())
	}
	return err
}
