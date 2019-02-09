package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"strings"
	"time"
	"whapp-irc/ircConnection"
	"whapp-irc/timestampMap"
	"whapp-irc/types"
	"whapp-irc/util"
	"whapp-irc/whapp"
)

// queue up to ten irc messages, this is especially helpful to answer PINGs in
// time, and during connection setup.
const ircMessageQueueSize = 10

// A Connection represents the internal state of a whapp-irc connection.
type Connection struct {
	WI    *whapp.Instance
	Chats *types.ChatList

	irc *ircConnection.Connection

	timestampMap *timestampMap.Map

	me           whapp.Me
	localStorage map[string]string
}

// BindSocket binds the given TCP connection.
func BindSocket(socket *net.TCPConn) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	irc := ircConnection.HandleConnection(ctx, socket)

	// when the irc connection dies or the context is cancelled, kill
	// everything off
	go func() {
		select {
		case <-irc.StopChannel():
		case <-ctx.Done():
		}
		cancel()
	}()

	// wait for the client to send a nickname
	select {
	case <-ctx.Done():
		return nil
	case <-irc.NickSetChannel():
	}

	if irc.Nick() == "" {
		return fmt.Errorf("nickname can't be empty")
	}

	// send welcome message
	if err := irc.WriteListNow([]string{
		fmt.Sprintf(":whapp-irc 001 %s :Welcome to whapp-irc, %s.", irc.Nick(), irc.Nick()),
		fmt.Sprintf(":whapp-irc 002 %s :Your host is whapp-irc.", irc.Nick()),
		fmt.Sprintf(":whapp-irc 003 %s :This server was created %s.", irc.Nick(), startTime),
		fmt.Sprintf(":whapp-irc 004 %s :", irc.Nick()),
		fmt.Sprintf(":whapp-irc 005 %s PREFIX=(qo)~@ CHARSET=UTF-8 :are supported by this server", irc.Nick()),
		fmt.Sprintf(":whapp-irc 375 %s :The server is running on commit %s", irc.Nick(), commit),
		fmt.Sprintf(":whapp-irc 372 %s :Enjoy the ride.", irc.Nick()),
		fmt.Sprintf(":whapp-irc 376 %s :End of /MOTD command.", irc.Nick()),
	}); err != nil {
		return err
	}

	var user types.User
	if found, err := userDb.GetItem(
		irc.Nick(),
		&user,
	); err == nil && found && user.Password != "" {
		// we've found an user with a password, so the current connection should
		// also provide a password.

		// passErr notifies the connection that the provided password is
		// incorrect, or none have been provided but should've been.
		passErr := func(goodErr error) error {
			msg := fmt.Sprintf(":whapp-irc 464 %s :Password incorrect", irc.Nick())
			if err := irc.WriteNow(msg); err != nil {
				return err
			}
			return goodErr
		}

		select {
		case <-ctx.Done():
			return nil

		case <-time.After(5 * time.Second):
			err := fmt.Errorf("password expected, but client timed out")
			return passErr(err)

		case <-irc.PassSetChannel():
			if irc.Pass() != user.Password {
				err := fmt.Errorf("client provided password incorrect")
				return passErr(err)
			}
		}
	}

	// setup bridge and connection
	conn, err := setupConnection(ctx, irc)
	if err != nil {
		irc.Status("error setting up whapp bridge: " + err.Error())
		return err
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

				if err := conn.handleIRCCommand(ctx, msg); err != nil {
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
	for _, item := range conn.Chats.List(false) {
		c := item.Chat

		prevTimestamp, found := conn.timestampMap.Get(c.ID)

		if empty || !conn.hasReplay() {
			conn.timestampMap.Set(c.ID, c.RawChat.Timestamp)
			continue
		} else if c.RawChat.Timestamp <= prevTimestamp {
			continue
		}

		if !found {
			// fetch all older messages
			prevTimestamp = math.MinInt64
		}

		messages, err := c.RawChat.GetMessagesFromChatTillDate(
			ctx,
			conn.WI,
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

			err := conn.handleWhappMessageReplay(ctx, msg)
			util.LogIfErr("error handling older whapp message", err)
		}
	}
	go conn.saveDatabaseEntry()

	conn.irc.Status("ready for new messages")

	// handle logging out on whatsapp web, this happens when the user removes
	// the bridge client on their phone.
	go func() {
		defer cancel()

		resCh, errCh := conn.WI.ListenLoggedIn(ctx, 3*time.Second)

		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				util.LogIfErr("error while listening for whatsapp loggedin state", err)
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

		messageCh, errCh := conn.WI.ListenForMessages(
			ctx,
			500*time.Millisecond,
		)
		queue := GetMessageQueue(ctx, messageCh, 50)

		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				util.LogIfErr("error while listening for whatsapp messages", err)
				return

			case msgRes := <-<-queue:
				if msgRes.Err == nil {
					msgRes.Err = conn.handleWhappMessage(
						ctx,
						msgRes.Message,
						handlerNormal,
					)
				}

				util.LogIfErr("error handling new whapp message", msgRes.Err)
			}
		}
	}()

	// now just wait until we have to shutdown.
	<-ctx.Done()
	log.Printf("connection ended: %s\n", ctx.Err())
	return nil
}

func (conn *Connection) joinChat(item types.ChatListItem) error {
	chat := item.Chat

	// sanity checks
	if chat == nil {
		return fmt.Errorf("chat is nil")
	} else if !chat.IsGroupChat {
		return fmt.Errorf("not a group chat")
	} else if chat.Joined {
		return nil
	}

	// identifier sanity check
	identifier := item.Identifier
	if identifier == "" || identifier == "#" {
		return fmt.Errorf("identifier is empty, chat.Name is %s", chat.Name)
	}

	// send JOIN to client
	str := fmt.Sprintf(":%s JOIN %s", conn.irc.Nick(), identifier)
	if err := conn.irc.WriteNow(str); err != nil {
		return err
	}

	// send chat name and description (if any) as topic
	topic := fmt.Sprintf(":whapp-irc 332 %s %s :%s", conn.irc.Nick(), identifier, chat.Name)
	if desc := chat.RawChat.Description; desc != nil {
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
				conn.irc.WriteNow(fmt.Sprintf(":whapp-irc MODE %s +q %s", identifier, conn.irc.Nick()))
			} else if participant.IsAdmin {
				conn.irc.WriteNow(fmt.Sprintf(":whapp-irc MODE %s +o %s", identifier, conn.irc.Nick()))
			}
			continue
		}

		prefix := ""
		if participant.IsSuperAdmin {
			prefix = "~"
		} else if participant.IsAdmin {
			prefix = "@"
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

func (conn *Connection) convertChat(
	chat whapp.Chat,
	participants []whapp.Participant,
) *types.Chat {
	converted := make([]types.Participant, len(participants))
	for i, p := range participants {
		converted[i] = types.Participant(p)
	}

	return &types.Chat{
		ID:   chat.ID,
		Name: chat.Title(),

		IsGroupChat:  chat.IsGroupChat,
		Participants: converted,

		RawChat: chat,
	}
}

func (conn *Connection) addChat(chat *types.Chat) types.ChatListItem {
	item, isNew := conn.Chats.Add(chat)
	if isNew {
		go conn.saveDatabaseEntry()
	}

	if item.Chat.IsGroupChat {
		log.Printf(
			"%-30s %3d participants\n",
			item.Identifier,
			len(item.Chat.Participants),
		)
	} else {
		log.Println(item.Identifier)
	}

	return item
}

func (conn *Connection) saveDatabaseEntry() error {
	err := userDb.SaveItem(conn.irc.Nick(), types.User{
		Password:             conn.irc.Pass(),
		LocalStorage:         conn.localStorage,
		LastReceivedReceipts: conn.timestampMap.GetCopy(),
		Chats:                conn.Chats.List(true),
	})
	util.LogIfErr("error while updating user entry", err)
	return err
}
