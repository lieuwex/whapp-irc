package main

import (
	"fmt"
	"strings"
	"time"
	"whapp-irc/ircConnection"
	"whapp-irc/util"
	"whapp-irc/whapp"
)

// Message represents a WhatsApp message, with some basic formatting for IRC.
type Message struct {
	From, To string
	Message  string
	IsReply  bool
	Original *whapp.Message
}

// Time returns the time the message has been sent.
func (msg Message) Time() time.Time {
	return msg.Original.Time()
}

// MessageHandler represents a handler for a WhatsApp message to be sent to an
// IRC client.
type MessageHandler func(conn *Connection, msg Message) error

var handlerNormal = func(conn *Connection, msg Message) error {
	lines := strings.Split(msg.Message, "\n")

	if msg.IsReply {
		line := "> " + lines[0]
		if nRest := len(lines) - 1; nRest > 0 {
			line = fmt.Sprintf(
				"%s [and %d more %s]",
				line,
				nRest,
				util.Plural(nRest, "line", "lines"),
			)
		}

		str := ircConnection.FormatPrivateMessage(msg.From, msg.To, line)
		return conn.irc.Write(msg.Time(), str)
	}

	for _, line := range lines {
		util.LogMessage(msg.Time(), msg.From, msg.To, line)
		str := ircConnection.FormatPrivateMessage(msg.From, msg.To, line)
		if err := conn.irc.Write(msg.Time(), str); err != nil {
			return err
		}
	}

	return nil
}

var handlerAlternativeReplay = func(conn *Connection, msg Message) error {
	if msg.IsReply {
		return nil
	}

	for _, line := range strings.Split(msg.Message, "\n") {
		util.LogMessage(msg.Time(), msg.From, msg.To, line)

		msg := fmt.Sprintf(
			"(%s) %s->%s: %s",
			msg.Time().Format("2006-01-02 15:04:05"),
			msg.From,
			msg.To,
			line,
		)
		str := ircConnection.FormatPrivateMessage("replay", conn.irc.Nick(), msg)
		if err := conn.irc.WriteNow(str); err != nil {
			return err
		}
	}

	return nil
}
