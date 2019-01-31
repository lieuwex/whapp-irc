package main

import (
	"fmt"
	"strings"
	"time"
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

		return conn.irc.PrivateMessage(msg.Time(), msg.From, msg.To, line)
	}

	for _, line := range lines {
		if err := conn.irc.PrivateMessage(
			msg.Time(),
			msg.From,
			msg.To,
			line,
		); err != nil {
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

		if err := conn.irc.PrivateMessage(
			time.Now(),
			"replay",
			conn.irc.Nick(),
			msg,
		); err != nil {
			return err
		}
	}

	return nil
}
