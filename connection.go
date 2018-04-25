package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"whapp-irc/whapp"

	"github.com/avast/retry-go"
	"github.com/mitchellh/mapstructure"
	qrcode "github.com/skip2/go-qrcode"
	irc "gopkg.in/sorcix/irc.v2"
	"gopkg.in/sorcix/irc.v2/ctcp"
)

func logMessage(from, to, message string) {
	log.Printf("%s->%s: %s", from, to, message)
}

var replyRegex = regexp.MustCompile(`^!(\d+)\s+(.+)$`)

type Connection struct {
	Chats []*Chat

	nickname string
	me       whapp.Me
	caps     []string

	bridge *Bridge
	socket *net.TCPConn

	messageCh <-chan whapp.Message
	errCh     <-chan error

	welcomed  bool
	welcomeCh chan bool

	waitch chan bool
}

func MakeConnection() (*Connection, error) {
	return &Connection{
		bridge: MakeBridge(),

		welcomeCh: make(chan bool),

		waitch: make(chan bool),
	}, nil
}

func (conn *Connection) BindSocket(socket *net.TCPConn) error {
	defer socket.Close()
	defer conn.bridge.Stop()

	conn.socket = socket
	write := conn.writeIRC
	status := conn.status

	closed := false
	closeWaitCh := func() {
		if !closed {
			close(conn.waitch)
			closed = true
		}
	}

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
				fmt.Printf("error while listening for IRC messages: %s\n", err.Error())
				closeWaitCh()
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

		conn.writeIRC(fmt.Sprintf(":whapp-irc 001 %s Welcome to whapp-irc, %s.", conn.nickname, conn.nickname))
		conn.writeIRC(fmt.Sprintf(":whapp-irc 002 %s Enjoy the ride.", conn.nickname))

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
				_, err := welcome()
				if err != nil {
					status("giving up trying to setup whapp bridge: " + err.Error())
					socket.Close()
					closeWaitCh()
					return
				}

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
				body := msg.Params[1]

				if tag, text, ok := ctcp.Decode(msg.Trailing()); ok && tag == ctcp.ACTION {
					body = fmt.Sprintf("_%s_", text)
				}

				logMessage(conn.nickname, to, body)

				if to == "status" {
					continue
				}

				chat := conn.GetChatByIdentifier(to)
				if chat == nil {
					status("unknown chat")
					continue
				}

				cid := chat.ID
				err := conn.bridge.WI.SendMessageToChatID(conn.bridge.ctx, cid, body)
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

			case "MODE":
				if len(msg.Params) != 3 {
					continue
				}

				ident := msg.Params[0]
				mode := msg.Params[1]
				nick := strings.ToLower(msg.Params[2])

				chat := conn.GetChatByIdentifier(ident)
				if chat == nil {
					status("chat not found")
					continue
				} else if mode != "+o" {
					continue
				}

				for _, p := range chat.Participants {
					if strings.ToLower(p.SafeName()) == nick {
						err := chat.rawChat.SetAdmin(conn.bridge.ctx, conn.bridge.WI, p.ID, true)
						if err != nil {
							str := fmt.Sprintf("error while opping %s: %s", nick, err.Error())
							status(str)
							fmt.Println(str)
							break
						}

						write(fmt.Sprintf(":%s MODE %s +o %s", conn.nickname, ident, nick))

						break
					}
				}

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

			case "WHO":
				identifier := msg.Params[0]
				chat := conn.GetChatByIdentifier(identifier)
				if chat != nil && chat.IsGroupChat {
					for _, p := range chat.Participants {
						presenceStamp := "H"
						presence, found, err := conn.getPresenceByUserID(p.ID)
						if found && err == nil && !presence.IsOnline {
							presenceStamp = "G"
						}

						msg := fmt.Sprintf(
							":whapp-irc 352 %s %s %s whapp-irc whapp-irc %s %s :0 %s",
							conn.nickname,
							identifier,
							p.SafeName(),
							p.SafeName(),
							presenceStamp,
							p.FullName(),
						)
						write(msg)
					}
				}
				write(fmt.Sprintf(":whapp-irc 315 %s %s :End of /WHO list.", conn.nickname, identifier))

			case "KICK":
				chatIdentifier := msg.Params[0]
				nick := strings.ToLower(msg.Params[1])

				chat := conn.GetChatByIdentifier(chatIdentifier)
				if chat == nil || !chat.IsGroupChat {
					write(fmt.Sprintf(":whapp-irc 403 %s %s :No such channel", conn.nickname, chatIdentifier))
					continue
				}

				for _, p := range chat.Participants {
					if strings.ToLower(p.SafeName()) == nick {
						err := chat.rawChat.RemoveParticipant(conn.bridge.ctx, conn.bridge.WI, p.ID)
						if err != nil {
							str := fmt.Sprintf("error while kicking %s: %s", nick, err.Error())
							status(str)
							fmt.Println(str)
						}
						break
					}
				}

			case "INVITE":
				nick := msg.Params[0]
				chatIdentifier := msg.Params[1]

				chat := conn.GetChatByIdentifier(chatIdentifier)
				if chat == nil || !chat.IsGroupChat {
					write(fmt.Sprintf(":whapp-irc 442 %s %s :You're not on that channel", conn.nickname, chatIdentifier))
					continue
				}
				personChat := conn.GetChatByIdentifier(nick)
				if personChat == nil || personChat.IsGroupChat {
					write(fmt.Sprintf(":whapp-irc 401 %s %s :No such nick/channel", conn.nickname, nick))
					continue
				}

				err := chat.rawChat.AddParticipant(conn.bridge.ctx, conn.bridge.WI, personChat.ID)
				if err != nil {
					str := fmt.Sprintf("error while adding %s: %s", nick, err.Error())
					status(str)
					fmt.Println(str)
					break
				}
			}
		}
	}()

	<-conn.welcomeCh


	go func() {
		resCh, errCh := conn.bridge.WI.ListenLoggedIn(conn.bridge.ctx, time.Second)

		for {
			var res bool

			select {
			case <-conn.waitch:
				return

			case err := <-errCh:
				fmt.Printf("error while listening for whatsapp loggedin state: %s\n", err.Error())
				closeWaitCh()
				return

			case res = <-resCh:
			}

			if res {
				continue
			}

			fmt.Println("logged out of whatsapp!")

			closeWaitCh()
			return
		}
	}()

	go func() {
		var err error

		for {
			var msg whapp.Message
			select {
			case <-conn.waitch:
				return

			case err := <-conn.errCh:
				fmt.Printf("error while listening for whatsapp messages: %s\n", err.Error())
				closeWaitCh()
				return

			case msg = <-conn.messageCh:
			}

			chat := conn.GetChatByID(msg.Chat.ID)
			if chat == nil {
				chat, err = conn.addChat(msg.Chat)
				if err != nil {
					fmt.Printf("err %s\n", err.Error())
					continue
				}
			}

			if chat.IsGroupChat && !chat.Joined {
				conn.joinChat(chat)
			}

			chat.AddMessageID(msg.ID.Serialized)

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

			_, ok := fs.HashToPath[msg.MediaFileHash]
			if msg.IsMMS && !ok {
				bytes, err := msg.DownloadMedia()
				if err != nil {
					fmt.Printf("err download %s\n", err.Error())
					continue
				}

				ext := getExtensionByMimeOrBytes(msg.MimeType, bytes)
				if ext == "" {
					ext = filepath.Ext(msg.MediaFilename)
					if ext != "" {
						ext = ext[1:]
					}
				}

				_, err = fs.AddBlob(
					msg.MediaFileHash,
					ext,
					bytes,
				)
				if err != nil {
					fmt.Printf("err addblob %s\n", err.Error())
					continue
				}
			}

			if msg.QuotedMessageObject != nil {
				message := getMessageBody(*msg.QuotedMessageObject, chat.Participants, conn.me)
				line := "> " + strings.SplitN(message, "\n", 2)[0]
				str := formatPrivateMessage(msg.Time(), senderSafeName, to, line)
				write(str)
			}

			message := getMessageBody(msg, chat.Participants, conn.me)
			for _, line := range strings.Split(message, "\n") {
				logMessage(senderSafeName, to, line)
				str := formatPrivateMessage(msg.Time(), senderSafeName, to, line)
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
	for _, participant := range chat.Participants {
		if participant.Contact.IsMe {
			if participant.IsAdmin {
				conn.writeIRC(fmt.Sprintf(":whapp-irc MODE %s +o %s", identifier, conn.nickname))
			}
			continue
		}

		prefix := ""
		if participant.IsAdmin {
			prefix = "@"
		}

		names = append(names, prefix+participant.SafeName())
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
	logMessage("status", conn.nickname, msg)
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
		var user User
		if err := mapstructure.Decode(obj, &user); err != nil {
			panic(err)
		}

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

	localStorage, err := conn.bridge.WI.GetLocalStorage(conn.bridge.ctx)
	if err != nil {
		fmt.Printf("error while getting local storage: %s\n", err.Error())
	} else {
		err := userDb.SaveItem(conn.nickname, User{
			Nickname:     conn.nickname,
			LocalStorage: localStorage,
		})
		if err != nil {
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

	conn.messageCh, conn.errCh = conn.bridge.WI.ListenForMessages(
		conn.bridge.ctx,
		500*time.Millisecond,
	)
	return nil
}

func getMessageBody(msg whapp.Message, participants []Participant, me whapp.Me) string {
	whappParticipants := make([]whapp.Participant, len(participants))
	for i, p := range participants {
		whappParticipants[i] = whapp.Participant(p)
	}

	if msg.Location != nil {
		return fmt.Sprintf(
			"https://maps.google.com/?q=%f,%f",
			msg.Location.Latitude,
			msg.Location.Longitude,
		)
	} else if msg.IsMMS {
		res := "--file--"
		if f, has := fs.HashToPath[msg.MediaFileHash]; has {
			res = f.URL
		}

		if msg.Caption != "" {
			res += " " + msg.FormatCaption(whappParticipants, me.Pushname)
		}

		return res
	}

	return msg.FormatBody(whappParticipants, me.Pushname)
}

func formatContact(contact whapp.Contact, isAdmin bool) Participant {
	return Participant{
		ID:      contact.ID,
		IsAdmin: isAdmin,
		Contact: contact,
	}
}

func formatPrivateMessage(date time.Time, from, to, line string) string {
	dateFormat := date.UTC().Format("2006-01-02T15:04:05.000Z")
	return fmt.Sprintf("@time=%s :%s PRIVMSG %s :%s", dateFormat, from, to, line)
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
