package main

import (
	"fmt"
	"strings"
	"time"

	irc "gopkg.in/sorcix/irc.v2"
	"gopkg.in/sorcix/irc.v2/ctcp"
)

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
	logMessage(time.Now(), "status", conn.nickname, msg)
	return conn.writeIRC(fmt.Sprintf(":status PRIVMSG %s :%s", conn.nickname, msg))
}

func formatPrivateMessage(date time.Time, from, to, line string) string {
	dateFormat := date.UTC().Format("2006-01-02T15:04:05.000Z")
	return fmt.Sprintf("@time=%s :%s PRIVMSG %s :%s", dateFormat, from, to, line)
}

func (conn *Connection) handleIRCCommand(msg *irc.Message) {
	write := conn.writeIRC
	status := conn.status

	switch msg.Command {
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

		logMessage(time.Now(), conn.nickname, to, body)

		if to == "status" {
			return
		}

		chat := conn.GetChatByIdentifier(to)
		if chat == nil {
			status("unknown chat")
			return
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
			return
		}
		err := conn.joinChat(chat)
		if err != nil {
			status("error while joining: " + err.Error())
		}

	case "PART":
		chat := conn.GetChatByIdentifier(msg.Params[0])
		if chat == nil {
			status("unknown chat")
			return
		}

		// TODO: some way that we don't rejoin a person later.
		chat.Joined = false

	case "MODE":
		if len(msg.Params) != 3 {
			return
		}

		ident := msg.Params[0]
		mode := msg.Params[1]
		nick := strings.ToLower(msg.Params[2])

		chat := conn.GetChatByIdentifier(ident)
		if chat == nil {
			status("chat not found")
			return
		} else if mode != "+o" {
			return
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
				if p.Contact.IsMe {
					continue
				}

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
			return
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
			return
		}
		personChat := conn.GetChatByIdentifier(nick)
		if personChat == nil || personChat.IsGroupChat {
			write(fmt.Sprintf(":whapp-irc 401 %s %s :No such nick/channel", conn.nickname, nick))
			return
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
