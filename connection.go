package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
	"whapp-irc/ircConnection"
	"whapp-irc/whapp"

	qrcode "github.com/skip2/go-qrcode"
)

// queue up to ten irc messages, this is especially helpful to answer PINGs in
// time, and during connection setup.
const ircMessageQueueSize = 10

var replyRegex = regexp.MustCompile(`^!(\d+)\s+(.+)$`)

// A Connection represents an IRC connection.
type Connection struct {
	Chats []*Chat

	me whapp.Me

	bridge *Bridge
	irc    *ircConnection.IRCConnection

	welcomed bool

	localStorage map[string]string

	timestampMap *TimestampMap
}

// BindSocket binds the given TCP connection.
func BindSocket(socket *net.TCPConn) error {
	ctx, cancel := context.WithCancel(context.Background())
	conn := &Connection{
		bridge: MakeBridge(),
		irc:    ircConnection.HandleConnection(ctx, socket),

		timestampMap: MakeTimestampMap(),
	}

	go func() {
		select {
		// when irc connection dies, cancel context
		case <-conn.irc.StopChannel():
			cancel()
		case <-ctx.Done():
		}

		// when the context is cancelled, kill everything off
		conn.irc.Close()
		conn.bridge.Stop()
	}()

	// welcome will send the welcome message to the user and setup the bridge.
	welcome := func() (setup bool, err error) {
		if conn.welcomed || conn.irc.Nick() == "" {
			return false, nil
		}

		if err := conn.irc.WriteListNow([]string{
			fmt.Sprintf(":whapp-irc 001 %s :Welcome to whapp-irc, %s.", conn.irc.Nick(), conn.irc.Nick()),
			fmt.Sprintf(":whapp-irc 002 %s :Your host is whapp-irc.", conn.irc.Nick()),
			fmt.Sprintf(":whapp-irc 003 %s :This server was created %s.", conn.irc.Nick(), startTime),
			fmt.Sprintf(":whapp-irc 004 %s :", conn.irc.Nick()),
			fmt.Sprintf(":whapp-irc 005 %s PREFIX=(oh)@%% CHARSET=UTF-8 :are supported by this server", conn.irc.Nick()),
			fmt.Sprintf(":whapp-irc 375 %s :The server is running on commit %s", conn.irc.Nick(), commit),
			fmt.Sprintf(":whapp-irc 372 %s :Enjoy the ride.", conn.irc.Nick()),
			fmt.Sprintf(":whapp-irc 376 %s :End of /MOTD command.", conn.irc.Nick()),
		}); err != nil {
			return false, err
		}

		conn.welcomed = true

		if err := conn.setup(cancel); err != nil {
			log.Printf("err while setting up: %s\n", err.Error())
			return false, err
		}

		return true, nil
	}

	// wait for the client to send a nickname
	select {
	case <-ctx.Done():
		cancel()
		return nil
	case <-conn.irc.NickSetChannel():
	}

	if _, err := welcome(); err != nil {
		conn.irc.Status("erroring setting up whapp bridge: " + err.Error())
		cancel()
		return nil
	}

	// now that we have set-up the bridge...

	// actually handle most of the IRC messages
	go func() {
		defer cancel()
		ircReceiveCh := conn.irc.ReceiveChannel()

		for {
			select {
			case <-ctx.Done():
				return

			case msg, ok := <-ircReceiveCh:
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
	// if negotiation hasn't started yet, we just skip through (we figure the
	// client doesn't support IRCv3, since normally negotiation occurs fairly
	// early in the connection)
	started, ok := conn.irc.Caps.WaitNegotiation(ctx)
	if !ok {
		return nil
	} else if !started {
		str := "IRCv3 capabilities negotiation has not started, " +
			"this is probably a non IRCv3 compatible client."
		log.Printf(str)
	}

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

	conn.irc.Status("ready for new messages")

	// handle logging out on whatsapp web, this happens when the user removes
	// the bridge client on their phone.
	go func() {
		defer cancel()

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

				conn.irc.Status("logged out of whatsapp")

				return
			}
		}
	}()

	// listen for new WhatsApp messages
	go func() {
		defer cancel()

		// REVIEW: we use other `ctx`s here, is that correct?
		// TODO: It looks like we should have to restart this, a bridge should
		// have closer grasp of whatever messages should be sent. Currently a
		// bridge is loosly defined of whatever it does. The struct itself
		// should provide more functions, and we should do less. In a prefect
		// world, WI isn't exposed.
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
	// sanity checks
	if chat == nil {
		return fmt.Errorf("chat is nil")
	} else if !chat.IsGroupChat {
		return fmt.Errorf("not a group chat")
	} else if chat.Joined {
		return nil
	}

	// identifier sanity check
	identifier := chat.Identifier()
	if identifier == "" || identifier == "#" {
		return fmt.Errorf("chat.Identifier() is empty, chat.Name is %s", chat.Name)
	}

	// send JOIN to client
	str := fmt.Sprintf(":%s JOIN %s", conn.irc.Nick(), identifier)
	if err := conn.irc.WriteNow(str); err != nil {
		return err
	}

	// send chat name and description (if any) as topic
	topic := fmt.Sprintf(":whapp-irc 332 %s %s :%s", conn.irc.Nick(), identifier, chat.Name)
	if desc := chat.rawChat.Description; desc != nil {
		if d := strings.TrimSpace(desc.Description); d != "" {
			d = strings.Replace(d, "\n", " ", -1)
			topic = fmt.Sprintf("%s: %s", topic, d)
		}
	}
	conn.irc.WriteNow(topic)

	// send chat members to client
	names := make([]string, 0)
	for _, participant := range chat.Participants {
		if participant.Contact.IsMe {
			if participant.IsSuperAdmin {
				conn.irc.WriteNow(fmt.Sprintf(":whapp-irc MODE %s +o %s", identifier, conn.irc.Nick()))
			} else if participant.IsAdmin {
				conn.irc.WriteNow(fmt.Sprintf(":whapp-irc MODE %s +h %s", identifier, conn.irc.Nick()))
			}
			continue
		}

		prefix := ""
		if participant.IsSuperAdmin {
			prefix = "@"
		} else if participant.IsAdmin {
			prefix = "%"
		}

		names = append(names, prefix+participant.SafeName())
	}
	str = fmt.Sprintf(":whapp-irc 353 %s @ %s :%s", conn.irc.Nick(), identifier, strings.Join(names, " "))
	if err := conn.irc.WriteNow(str); err != nil {
		return err
	}
	str = fmt.Sprintf(":whapp-irc 366 %s %s :End of /NAMES list.", conn.irc.Nick(), identifier)
	if err := conn.irc.WriteNow(str); err != nil {
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

func (conn *Connection) addChat(chat *Chat) {
	if chat.IsGroupChat {
		log.Printf("%-30s %3d participants\n", chat.Identifier(), len(chat.Participants))
	} else {
		log.Println(chat.Identifier())
	}

	for i, c := range conn.Chats {
		if c.ID == chat.ID {
			conn.Chats[i] = chat
			return
		}
	}
	conn.Chats = append(conn.Chats, chat)
}

// TODO: check if already set-up
func (conn *Connection) setup(cancel context.CancelFunc) error {
	if _, err := conn.bridge.Start(); err != nil {
		return err
	}

	go func() {
		// this is actually kind rough, but it seems to work better
		// currently...
		<-conn.bridge.ctx.Done()
		cancel()
	}()

	// if we have the current user in the database, try to relogin using the
	// previous localStorage state
	var user User
	found, err := userDb.GetItem(conn.irc.Nick(), &user)
	if err != nil {
		return err
	} else if found {
		conn.timestampMap.Swap(user.LastReceivedReceipts)

		conn.irc.Status("logging in using stored session")

		if err := conn.bridge.WI.Navigate(conn.bridge.ctx); err != nil {
			return err
		}
		if err := conn.bridge.WI.SetLocalStorage(
			conn.bridge.ctx,
			user.LocalStorage,
		); err != nil {
			log.Printf("error while setting local storage: %s\n", err.Error())
		}
	}

	// open site
	state, err := conn.bridge.WI.Open(conn.bridge.ctx)
	if err != nil {
		return err
	}

	// if we aren't logged in yet we have to get the QR code and stuff
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

		if err := conn.irc.Status("Scan this QR code: " + qrFile.URL); err != nil {
			return err
		}
	}

	// waiting for login
	if err := conn.bridge.WI.WaitLogin(conn.bridge.ctx); err != nil {
		return err
	}
	conn.irc.Status("logged in")

	// get localstorage (that contains new login information), and save it to
	// the database
	conn.localStorage, err = conn.bridge.WI.GetLocalStorage(conn.bridge.ctx)
	if err != nil {
		log.Printf("error while getting local storage: %s\n", err.Error())
	} else {
		if err := conn.saveDatabaseEntry(); err != nil {
			return err
		}
	}

	// get information about the user
	conn.me, err = conn.bridge.WI.GetMe(conn.bridge.ctx)
	if err != nil {
		return err
	}

	// get raw chats
	rawChats, err := conn.bridge.WI.GetAllChats(conn.bridge.ctx)
	if err != nil {
		return err
	}

	// convert chats to internal reprenstation, we do this using a second slice
	// and a WaitGroup to preserve the initial order
	chats := make([]*Chat, len(rawChats))
	var wg sync.WaitGroup
	for i, raw := range rawChats {
		wg.Add(1)
		go func(i int, raw whapp.Chat) {
			defer wg.Done()

			chat, err := conn.convertChat(raw)
			if err != nil {
				str := fmt.Sprintf("error while converting chat with ID %s, skipping", raw.ID)
				conn.irc.Status(str)
				log.Printf("%s. error: %s", str, err)
				return
			}

			chats[i] = chat
		}(i, raw)
	}
	wg.Wait()

	// add all chats to connection
	for _, chat := range chats {
		if chat == nil {
			// there was an error converting this chat, skip it.
			continue
		}

		conn.addChat(chat)
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
	err := userDb.SaveItem(conn.irc.Nick(), User{
		LocalStorage:         conn.localStorage,
		LastReceivedReceipts: conn.timestampMap.GetCopy(),
	})
	if err != nil {
		log.Printf("error while updating user entry: %s\n", err)
	}
	return err
}

func (conn *Connection) hasReplay() bool {
	return conn.irc.Caps.Has("whapp-irc/replay") || alternativeReplay
}
