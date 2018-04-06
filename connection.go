package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"
	"whapp-irc/whapp"

	"github.com/avast/retry-go"
	qrcode "github.com/skip2/go-qrcode"
	irc "gopkg.in/sorcix/irc.v2"
)

var replyRegex = regexp.MustCompile(`^!(\d+)\s+(.+)$`)

type Connection struct {
	Chats []*Chat

	nickname  string
	me        whapp.Me
	welcommed bool
	caps      []string

	bridge    *Bridge
	socket    *net.TCPConn
	messageCh chan whapp.Message

	waitch chan bool
}

func MakeConnection() (*Connection, error) {
	return &Connection{
		bridge: MakeBridge(),

		waitch:    make(chan bool),
		messageCh: make(chan whapp.Message),
	}, nil
}

func (conn *Connection) BindSocket(socket *net.TCPConn) error {
	conn.socket = socket
	write := conn.writeIRC
	status := conn.status

	ircCh := make(chan *irc.Message)

	// listen for and parse messages.
	// we want to do this outside the next irc message handle loop, so we can
	// reply to PINGs but not handle stuff like JOINs yet.
	go func() {
		decoder := irc.NewDecoder(bufio.NewReader(socket))
		for {
			msg, err := decoder.Decode()
			if err != nil {
				log.Printf("%#v", err)
				close(conn.waitch)
				return
			}
			fmt.Printf("%s\t%#v\n", conn.nickname, msg)

			if msg.Command == "PING" {
				write(":whapp-irc PONG whapp-irc :" + msg.Params[0])
				continue
			}

			ircCh <- msg
		}
	}()

	welcome := func() {
		if conn.welcommed || conn.nickname == "" {
			return
		}

		conn.writeIRC(fmt.Sprintf(":whapp-irc 001 %s Welcome to whapp-irc, %s.", conn.nickname, conn.nickname))
		conn.writeIRC(fmt.Sprintf(":whapp-irc 002 %s Enjoy the ride.", conn.nickname))

		conn.welcommed = true

		err := retry.Do(func() error {
			conn.bridge.Stop()
			err := conn.setup()
			if err != nil {
				fmt.Printf("err while setting up: %s\n", err.Error())
			}
			return err
		}, retry.Attempts(5), retry.Delay(time.Second))
		if err != nil {
			panic(err) // REVIEW
		}
	}

	go func() {
		for {
			var msg *irc.Message
			select {
			case <-conn.waitch:
				return
			case msg = <-ircCh:
			}

			switch msg.Command {
			case "NICK":
				conn.nickname = msg.Params[0]
				welcome()

			case "CAP":
				switch msg.Params[0] {
				case "LS":
					write(":whapp-irc CAP * LS :server-time")

				case "REQ":
					caps := strings.Split(msg.Trailing(), " ")
					for _, cap := range caps {
						conn.caps = append(conn.caps, strings.TrimSpace(cap))
					}
				}

			case "PRIVMSG":
				to := msg.Params[0]
				msg := msg.Params[1]

				if to == "status" {
					fmt.Printf("%s->status: %s\n", conn.nickname, msg)
					continue
				}

				chat := conn.GetChatByIdentifier(to)
				if chat == nil {
					status("unknown chat")
					continue
				}

				cid := chat.ID
				err := conn.bridge.WI.SendMessageToChatID(conn.bridge.ctx, cid, msg)
				if err != nil {
					fmt.Printf("err while sending %s\n", err.Error())
				}

			case "JOIN":
				chat := conn.GetChatByIdentifier(msg.Params[0])
				if chat == nil {
					status("chat not found")
					continue
				}
				err := conn.joinChat(chat)
				if err != nil {
					status("error while joining: " + err.Error())
				}

			case "PART":
				chat := conn.GetChatByIdentifier(msg.Params[0])
				if chat == nil {
					status("unknown chat")
					continue
				}

				// TODO: some way that we don't rejoin a person later.
				chat.Joined = false

			case "LIST":
				// TODO: support args
				for _, c := range conn.Chats {
					nParticipants := len(c.Participants)
					if !c.IsGroupChat {
						nParticipants = 2
					}

					write(fmt.Sprintf(":whapp-irc 322 %s %s %d :%s", conn.nickname, c.Identifier(), nParticipants, c.Name))
				}
				write(fmt.Sprintf(":whapp-irc 323 %s :End of LIST", conn.nickname))
			}
		}
	}()

	go func() {
		var err error

		for {
			msg := <-conn.messageCh
			chat := conn.GetChatByID(msg.Chat.ID)
			if chat == nil {
				chat, err = conn.addChat(&msg.Chat)
				if err != nil {
					fmt.Printf("err %s\n", err.Error())
					continue
				}
			}

			if chat.IsGroupChat && !chat.Joined {
				conn.joinChat(chat)
			}

			chat.AddMessageID(msg.ID.Serialized)
			fmt.Printf("\t%#v\n", msg)

			sender := formatContact(msg.Sender, false)
			senderSafeName := sender.SafeName()

			if msg.IsSentByMeFromWeb {
				continue
			} else if msg.IsSentByMe {
				senderSafeName = conn.nickname
			}

			var to string
			if chat.IsGroupChat || msg.IsSentByMe {
				to = chat.Identifier()
			} else {
				to = conn.nickname
			}

			if msg.IsMedia {
				bytes, err := msg.DownloadMedia()
				if err != nil {
					fmt.Printf("err download %s\n", err.Error())
					continue
				}
				_, err = fs.AddBlob(msg.ID.Serialized, getFileName(bytes), bytes)
				if err != nil {
					fmt.Printf("err addblob %s\n", err.Error())
					continue
				}
			}
			message := getMessageBody(msg)

			date := msg.Time().UTC().Format("2006-01-02T15:04:05.000Z")

			if msg.QuotedMessageObject != nil {
				line := "> " + strings.SplitN(msg.QuotedMessageObject.Content(), "\n", 2)[0]
				str := fmt.Sprintf("@time=%s :%s PRIVMSG %s :%s", date, senderSafeName, to, line)
				write(str)
			}

			for _, line := range strings.Split(message, "\n") {
				fmt.Printf("\t%s: %s\n", sender.FullName(), line)
				str := fmt.Sprintf("@time=%s :%s PRIVMSG %s :%s", date, senderSafeName, to, line)
				write(str)
			}
		}
	}()

	<-conn.waitch
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

	conn.writeIRC(fmt.Sprintf(":%s JOIN %s", conn.nickname, identifier))
	conn.writeIRC(fmt.Sprintf(":whapp-irc 332 %s %s :%s", conn.nickname, identifier, chat.Name))

	names := make([]string, 0)
	for _, contact := range chat.Participants {
		if contact.IsMe {
			if contact.IsAdmin {
				conn.writeIRC(fmt.Sprintf(":whapp-irc MODE %s +o %s", identifier, conn.nickname))
			}
			continue
		}

		prefix := ""
		if contact.IsAdmin {
			prefix = "@"
		}

		names = append(names, prefix+contact.SafeName())
	}

	conn.writeIRC(fmt.Sprintf(":whapp-irc 353 %s @ %s :%s", conn.nickname, identifier, strings.Join(names, " ")))
	conn.writeIRC(fmt.Sprintf(":whapp-irc 366 %s %s :End of /NAMES list.", conn.nickname, identifier))

	chat.Joined = true
	return nil
}

func (conn *Connection) writeIRC(msg string) error {
	bytes := []byte(msg + "\n")

	n, err := conn.socket.Write(bytes)
	if err != nil {
		return err
	} else if n != len(bytes) {
		return fmt.Errorf("bytes length mismatch")
	}

	return nil
}

func (conn *Connection) status(msg string) error {
	return conn.writeIRC(fmt.Sprintf(":status PRIVMSG %s :%s", conn.nickname, msg))
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

func formatContact(contact whapp.Contact, isAdmin bool) Contact {
	return Contact{
		ID:      contact.ID,
		IsAdmin: isAdmin,
		IsMe:    contact.IsMe,

		Names: ContactNames{
			Short:     contact.ShortName,
			Push:      contact.PushName,
			Formatted: contact.FormattedName,
		},
	}
}

func (conn *Connection) addChat(chat *whapp.Chat) (*Chat, error) {
	participants, err := chat.Participants(conn.bridge.ctx, conn.bridge.WI)
	if err != nil {
		return nil, err
	}

	converted := make([]Contact, len(participants))
	for i, p := range participants {
		converted[i] = formatContact(p.Contact, p.IsAdmin)
	}

	res := &Chat{
		ID:   chat.ID,
		Name: chat.Name,

		IsGroupChat:  chat.IsGroupChat,
		Participants: converted,

		Joined:     false,
		MessageIDs: make([]string, 0),

		rawChat: chat,
	}

	fmt.Printf("%s\t\t\t\t%d participants\n", res.Identifier(), len(res.Participants))

	for i, c := range conn.Chats {
		if c.ID == chat.ID {
			conn.Chats[i] = res
			goto done
		}
	}
	conn.Chats = append(conn.Chats, res)
	goto done

done:
	return res, nil
}

// TODO: check if already setup
func (conn *Connection) setup() error {
	_, err := conn.bridge.Start()
	if err != nil {
		return err
	}

	state, err := conn.bridge.WI.Open(conn.bridge.ctx)
	if err != nil {
		return err
	}

	var qrFile *File
	if state == whapp.Loggedout {
		code, err := conn.bridge.WI.GetLoginCode(conn.bridge.ctx)
		if err != nil {
			return fmt.Errorf("qr code not loaded correctly or smth")
		}

		bytes, err := qrcode.Encode(code, qrcode.High, 512)
		if err != nil {
			return err
		}

		qrFile, err = fs.AddBlob("", "qr-"+strTimestamp(), bytes)
		if err != nil {
			return err
		}

		conn.status("Scan this QR code: " + qrFile.URL)
	}

	if err := conn.bridge.WI.WaitLogin(conn.bridge.ctx); err != nil {
		return err
	}
	conn.status("logged in")

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

	return conn.bridge.WI.ListenForMessages(
		conn.bridge.ctx,
		conn.messageCh,
		500*time.Millisecond,
	)
}

func getMessageBody(msg whapp.Message) string {
	res := msg.Body

	if msg.IsMedia {
		res = "-- file --"
		if f := fs.IDToPath[msg.ID.Serialized]; f != nil {
			res = f.URL
		}

		if msg.Caption != "" {
			res += " " + msg.Caption
		}
	}

	return res
}
