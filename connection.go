package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"regexp"
	"strings"
	"time"
	"whapp-irc/capabilities"
	"whapp-irc/whapp"

	"github.com/avast/retry-go"
	qrcode "github.com/skip2/go-qrcode"
	irc "gopkg.in/sorcix/irc.v2"
)

var replyRegex = regexp.MustCompile(`^!(\d+)\s+(.+)$`)

// A Connection represents an IRC connection.
type Connection struct {
	Chats []*Chat

	nickname string
	me       whapp.Me

	caps *capabilities.CapabilitiesMap

	bridge *Bridge
	socket *net.TCPConn
	cancel context.CancelFunc

	welcomed bool

	localStorage map[string]string

	timestampMap *TimestampMap
}

// MakeConnection returns a new Connection instance.
func MakeConnection() (*Connection, error) {
	return &Connection{
		bridge: MakeBridge(),

		caps:         capabilities.MakeCapabilitiesMap(),
		timestampMap: MakeTimestampMap(),
	}, nil
}

func (conn *Connection) stop() {
	conn.socket.Close()
	conn.bridge.Stop()
	conn.cancel()
}

// listen for and parse messages.
// this function also handles IRC commands which are independent of the rest of
// whapp-irc, such as PINGs.
func (conn *Connection) listenIRC(ctx context.Context) <-chan *irc.Message {
	ircCh := make(chan *irc.Message)

	go func() {
		defer close(ircCh)

		write := conn.writeIRCNow
		decoder := irc.NewDecoder(bufio.NewReader(conn.socket))
		for {
			msg, err := decoder.Decode()
			if err == io.EOF {
				return
			} else if err != nil {
				log.Printf("error while listening for IRC messages: %s\n", err)
				return
			}

			if msg == nil {
				log.Println("got invalid IRC message, ignoring")
				continue
			}

			switch msg.Command {
			case "PING":
				str := ":whapp-irc PONG whapp-irc :" + msg.Params[0]
				if err := conn.writeIRCNow(str); err != nil {
					log.Printf("error while sending PONG: %s", err)
					return
				}
			case "QUIT":
				log.Printf("received QUIT from %s", conn.nickname)
				return

			case "CAP":
				conn.caps.StartNegotiation()
				switch msg.Params[0] {
				case "LS":
					write(":whapp-irc CAP * LS :server-time whapp-irc/replay")

				case "LIST":
					caps := conn.caps.Caps()
					write(":whapp-irc CAP * LIST :" + strings.Join(caps, " "))

				case "REQ":
					for _, cap := range strings.Split(msg.Trailing(), " ") {
						conn.caps.AddCapability(cap)
					}
					caps := conn.caps.Caps()
					write(":whapp-irc CAP * ACK :" + strings.Join(caps, " "))

				case "END":
					conn.caps.FinishNegotiation()
				}

			default:
				ircCh <- msg
			}
		}
	}()

	return ircCh
}

// BindSocket binds the given TCP connection to the current Connection instance.
func (conn *Connection) BindSocket(socket *net.TCPConn) error {
	defer conn.stop()
	ctx := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		conn.socket = socket
		conn.cancel = cancel
		return ctx
	}()

	ircCh := conn.listenIRC(ctx)

	// welcome will send the welcome message to the user and setup the bridge.
	welcome := func() (setup bool, err error) {
		if conn.welcomed || conn.nickname == "" {
			return false, nil
		}

		if err := conn.writeIRCListNow([]string{
			fmt.Sprintf(":whapp-irc 001 %s :Welcome to whapp-irc, %s.", conn.nickname, conn.nickname),
			fmt.Sprintf(":whapp-irc 002 %s :Your host is whapp-irc.", conn.nickname),
			fmt.Sprintf(":whapp-irc 003 %s :This server was created %s.", conn.nickname, startTime),
			fmt.Sprintf(":whapp-irc 004 %s :", conn.nickname),
			fmt.Sprintf(":whapp-irc 375 %s :The server is running on commit %s", conn.nickname, commit),
			fmt.Sprintf(":whapp-irc 372 %s :Enjoy the ride.", conn.nickname),
			fmt.Sprintf(":whapp-irc 376 %s :End of /MOTD command.", conn.nickname),
		}); err != nil {
			return false, err
		}

		conn.welcomed = true

		err = retry.Do(func() error {
			conn.bridge.Stop()
			err := conn.setup()
			if err != nil {
				log.Printf("err while setting up: %s\n", err.Error())
			}
			return err
		}, retry.Attempts(5), retry.Delay(time.Second))
		if err != nil {
			return false, err
		}

		return true, nil
	}

	// wait for the client to send a nickname
nickWait:
	for {
		select {
		case <-ctx.Done():
			conn.stop()
			return ctx.Err()
		case msg, ok := <-ircCh:
			if !ok {
				conn.stop()
				return ctx.Err()
			}

			if msg.Command != "NICK" {
				log.Printf("unexpected IRC command, expected NICK, got %s", msg.Command)
				continue
			}

			conn.nickname = msg.Params[0]
			if _, err := welcome(); err != nil {
				conn.status("giving up trying to setup whapp bridge: " + err.Error())
				conn.stop()
				return ctx.Err()
			}

			break nickWait
		}
	}

	// now that we have set-up the bridge...

	// actually handle most of the IRC messages
	go func() {
		defer conn.cancel()

		for {
			select {
			case <-ctx.Done():
				return

			case msg, ok := <-ircCh:
				if !ok {
					return
				}

				if err := conn.handleIRCCommand(msg); err != nil {
					log.Printf("error handling new irc message: %s\n", err)

					if err == io.ErrClosedPipe {
						return
					}
					continue
				}
			}
		}
	}()

	// we want to wait until we've finished negotiation, since when we send a
	// replay we want to know if the user has servertime and even if they want a
	// replay at all.
	conn.caps.WaitNegotiation()

	// replay older messages
	empty := conn.timestampMap.Length() == 0
	for _, c := range conn.Chats {
		prevTimestamp, found := conn.timestampMap.Get(c.ID.String())

		if empty || !conn.hasReplay() {
			conn.timestampMap.Set(c.ID.String(), c.rawChat.Timestamp)
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
			log.Printf("error while loading earlier messages: %s\n", err.Error())
			return err
		}

		for _, msg := range messages {
			if msg.Timestamp <= prevTimestamp {
				continue
			}

			if err := conn.handleWhappMessageReplay(msg); err != nil {
				log.Printf("error handling older whapp message: %s\n", err.Error())
				continue
			}
		}
	}
	conn.status("ready for new messages")

	// handle logging out on whatsapp web, this happens when the user removes
	// the bridge client on their phone.
	go func() {
		defer conn.stop()

		resCh, errCh := conn.bridge.WI.ListenLoggedIn(conn.bridge.ctx, time.Second)

		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				log.Printf("error while listening for whatsapp loggedin state: %s\n", err.Error())
				return

			case res := <-resCh:
				if res {
					continue
				}

				log.Println("logged out of whatsapp!")

				return
			}
		}
	}()

	// listen for new WhatsApp messages
	go func() {
		defer conn.stop()

		// REVIEW: we use other `ctx`s here, is that correct?
		messageCh, errCh := conn.bridge.WI.ListenForMessages(
			conn.bridge.ctx,
			500*time.Millisecond,
		)
		queue := GetMessageQueue(ctx, messageCh, 50)

		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				log.Printf("error while listening for whatsapp messages: %s\n", err.Error())
				return

			case msgFut := <-queue:
				msgRes := <-msgFut
				if msgRes.Err == nil {
					msgRes.Err = conn.handleWhappMessage(msgRes.Message)
				}

				if msgRes.Err != nil {
					log.Printf("error handling new whapp message: %s\n", msgRes.Err)
					continue
				}
			}

		}
	}()

	// now just wait until we have to shutdown.
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
	if identifier == "" || identifier == "#" {
		return fmt.Errorf("chat.Identifier() is empty, chat.Name is %s", chat.Name)
	}

	str := fmt.Sprintf(":%s JOIN %s", conn.nickname, identifier)
	if err := conn.writeIRCNow(str); err != nil {
		return err
	}

	topic := fmt.Sprintf(":whapp-irc 332 %s %s :%s", conn.nickname, identifier, chat.Name)
	if desc := chat.rawChat.Description; desc != nil {
		if d := strings.TrimSpace(desc.Description); d != "" {
			d = strings.Replace(d, "\n", " ", -1)
			topic = fmt.Sprintf("%s: %s", topic, d)
		}
	}
	if err := conn.writeIRCNow(topic); err != nil {
		return err
	}

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

	str = fmt.Sprintf(":whapp-irc 353 %s @ %s :%s", conn.nickname, identifier, strings.Join(names, " "))
	if err := conn.writeIRCNow(str); err != nil {
		return err
	}
	str = fmt.Sprintf(":whapp-irc 366 %s %s :End of /NAMES list.", conn.nickname, identifier)
	if err := conn.writeIRCNow(str); err != nil {
		return err
	}

	chat.Joined = true
	return nil
}

// GetChatByID returns the chat with the given ID, if any.
func (conn *Connection) GetChatByID(ID whapp.ID) *Chat {
	for _, c := range conn.Chats {
		if c.ID == ID {
			return c
		}
	}
	return nil
}

// GetChatByIdentifier returns the chat with the given identifier, if any.
func (conn *Connection) GetChatByIdentifier(identifier string) *Chat {
	identifier = strings.ToLower(identifier)

	for _, c := range conn.Chats {
		if strings.ToLower(c.Identifier()) == identifier {
			return c
		}
	}
	return nil
}

func (conn *Connection) convertChat(chat whapp.Chat) (*Chat, error) {
	participants, err := chat.Participants(conn.bridge.ctx, conn.bridge.WI)
	if err != nil {
		return nil, err
	}

	converted := make([]Participant, len(participants))
	for i, p := range participants {
		converted[i] = Participant(p)
	}

	return &Chat{
		ID:   chat.ID,
		Name: chat.Title(),

		IsGroupChat:  chat.IsGroupChat,
		Participants: converted,

		Joined:     false,
		MessageIDs: make([]string, 0),

		rawChat: chat,
	}, nil
}

func (conn *Connection) addChat(rawChat whapp.Chat) (*Chat, error) {
	chat, err := conn.convertChat(rawChat)
	if err != nil {
		return nil, err
	}

	if chat.IsGroupChat {
		log.Printf("%-30s %3d participants\n", chat.Identifier(), len(chat.Participants))
	} else {
		log.Println(chat.Identifier())
	}

	for i, c := range conn.Chats {
		if c.ID == chat.ID {
			conn.Chats[i] = chat
			return chat, nil
		}
	}
	conn.Chats = append(conn.Chats, chat)
	return chat, nil
}

// TODO: check if already set-up
func (conn *Connection) setup() error {
	if _, err := conn.bridge.Start(); err != nil {
		return err
	}

	var user User
	found, err := userDb.GetItem(conn.nickname, &user)
	if err != nil {
		return err
	} else if found {
		conn.timestampMap.Swap(user.LastReceivedReceipts)

		if _, err := conn.bridge.WI.Open(conn.bridge.ctx); err != nil {
			return err
		}

		if err := conn.bridge.WI.SetLocalStorage(
			conn.bridge.ctx,
			user.LocalStorage,
		); err != nil {
			log.Printf("error while setting local storage: %s\n", err.Error())
		}
	}

	state, err := conn.bridge.WI.Open(conn.bridge.ctx)
	if err != nil {
		return err
	}

	if state == whapp.Loggedout {
		code, err := conn.bridge.WI.GetLoginCode(conn.bridge.ctx)
		if err != nil {
			return fmt.Errorf("Error while retrieving login code: %s", err.Error())
		}

		bytes, err := qrcode.Encode(code, qrcode.High, 512)
		if err != nil {
			return err
		}

		qrFile, err := fs.AddBlob("qr-"+strTimestamp(), "png", bytes)
		if err != nil {
			return err
		}
		defer func() {
			if err = fs.RemoveFile(qrFile); err != nil {
				log.Printf("error while removing QR code: %s\n", err.Error())
			}
		}()

		if err := conn.status("Scan this QR code: " + qrFile.URL); err != nil {
			return err
		}
	}

	if err := conn.bridge.WI.WaitLogin(conn.bridge.ctx); err != nil {
		return err
	}
	conn.status("logged in")

	conn.localStorage, err = conn.bridge.WI.GetLocalStorage(conn.bridge.ctx)
	if err != nil {
		log.Printf("error while getting local storage: %s\n", err.Error())
	} else {
		if err := conn.saveDatabaseEntry(); err != nil {
			return err
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
			str := fmt.Sprintf("error while converting chat with ID %s, skipping", chat.ID)
			conn.status(str)
			log.Printf(str + " error: " + err.Error())
			continue
		}
	}

	return nil
}

func (conn *Connection) getPresenceByUserID(userID whapp.ID) (presence whapp.Presence, found bool, err error) {
	for _, c := range conn.Chats {
		if c.ID == userID {
			presence, err := c.rawChat.GetPresence(conn.bridge.ctx, conn.bridge.WI)
			return presence, true, err
		}
	}

	return whapp.Presence{}, false, nil
}

func (conn *Connection) saveDatabaseEntry() error {
	err := userDb.SaveItem(conn.nickname, User{
		LocalStorage:         conn.localStorage,
		LastReceivedReceipts: conn.timestampMap.GetCopy(),
	})
	if err != nil {
		log.Printf("error while updating user entry: %s\n", err)
	}
	return err
}

func (conn *Connection) hasReplay() bool {
	return conn.caps.HasCapability("whapp-irc/replay") || alternativeReplay
}
