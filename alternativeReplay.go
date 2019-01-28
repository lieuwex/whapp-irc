package main

import (
	"fmt"
	"strings"
	"whapp-irc/ircConnection"
	"whapp-irc/whapp"
)

func (conn *Connection) alternativeReplayWhappMessageHandle(msg whapp.Message) error {
	item, has := conn.GetChatByID(msg.Chat.ID)
	if !has {
		chat, err := conn.convertChat(msg.Chat)
		if err != nil {
			return err
		}
		item = conn.addChat(chat)
	}
	chat := item.chat

	if chat.HasMessageID(msg.ID.Serialized) {
		return nil // already handled
	}
	chat.AddMessageID(msg.ID.Serialized)

	lastTimestamp, found := conn.timestampMap.Get(chat.ID.String())
	if !found || msg.Timestamp > lastTimestamp {
		conn.timestampMap.Set(chat.ID.String(), msg.Timestamp)
		go conn.saveDatabaseEntry()
	}

	if msg.IsNotification {
		return nil
	}

	sender := formatContact(*msg.Sender)
	from := sender.SafeName()
	if msg.IsSentByMe {
		from = conn.irc.Nick()
	}

	var to string
	if chat.IsGroupChat || msg.IsSentByMe {
		to = item.Identifier
	} else {
		to = conn.irc.Nick()
	}

	if err := downloadAndStoreMedia(msg); err != nil {
		return err
	}

	message := getMessageBody(msg, chat.Participants, conn.me)
	for _, line := range strings.Split(message, "\n") {
		logMessage(msg.Time(), from, to, line)

		msg := fmt.Sprintf(
			"(%s) %s->%s: %s",
			msg.Time().Format("2006-01-02 15:04:05"),
			from,
			to,
			line,
		)
		str := ircConnection.FormatPrivateMessage("replay", conn.irc.Nick(), msg)
		if err := conn.irc.WriteNow(str); err != nil {
			return err
		}
	}

	return nil
}
