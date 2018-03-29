package main

import (
	"regexp"
	"strings"
	"time"
)

const MessageIDListSize = 750

var numberRegex = regexp.MustCompile(`^\+[\d ]+$`)
var nonNumberRegex = regexp.MustCompile(`[^\d]`)

type Command struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type Event struct {
	Event string                   `json:"event"`
	Args  []map[string]interface{} `json:"args"`
}

type ContactNames struct {
	Short     string `json:"short"`
	Push      string `json:"push"`
	Formatted string `json:"formatted"`
}

type Contact struct {
	ID    string       `json:"id"`
	Names ContactNames `json:"names"`

	IsAdmin bool
}

func (c *Contact) FullName() string {
	return c.Names.Formatted
}

func (c *Contact) SafeName() string {
	str := c.FullName()
	if numberRegex.MatchString(str) && IRCsafeString(c.Names.Push) != "" {
		str = c.Names.Push
	}

	return IRCsafeString(str)
}

func (c *Contact) Self(number string) bool {
	number = nonNumberRegex.ReplaceAllLiteralString(number, "")
	return strings.Contains(c.ID, number)
}

type Chat struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Participants []Contact `json:"participants"`
	Admins       []Contact `json:"admins"`

	Joined     bool
	MessageIDs []string
}

func (c *Chat) IsGroupChat() bool {
	return len(c.Participants) > 0
}

func (c *Chat) SafeName() string {
	return IRCsafeString(c.Name)
}

func (c *Chat) Identifier() string {
	prefix := ""
	if c.IsGroupChat() {
		prefix = "#"
	}
	return prefix + c.SafeName()
}

func (c *Chat) AddMessageID(id string) {
	if len(c.MessageIDs) >= MessageIDListSize {
		c.MessageIDs = c.MessageIDs[1:]
	}
	c.MessageIDs = append(c.MessageIDs, id)
}

type Message struct {
	ID        string  `json:"id"`
	Timestamp int64   `json:"timestamp"`
	Sender    Contact `json:"sender"`
	Body      string  `json:"body"`

	IsSentByMe        bool `json:"isSentByMe"`
	IsSentByMeFromWeb bool `json:"isSentByMeFromWeb"`

	IsMedia        bool `json:"isMedia"`
	IsNotification bool `json:"isNotif"`
	IsText         bool `json:"isText"`

	QuotedMessageObject *Message `json:"quotedMsgObj" mapstructure:"quotedMsgObj"`

	Filename string                 `json:"filename"`
	Caption  string                 `json:"caption"`
	Keys     map[string]interface{} `json:"keys"`
}

func (msg *Message) Content() string {
	res := msg.Body

	if msg.IsMedia {
		res = "-- file --"
		if f := fs.IDToPath[msg.ID]; f != nil {
			res = f.URL
		}

		if msg.Caption != "" {
			res += " " + msg.Caption
		}
	}

	return res
}

func (msg *Message) Time() time.Time {
	return time.Unix(msg.Timestamp, 0)
}

type MessageGroup struct {
	Chat     Chat       `json:"chat"`
	Messages []*Message `json:"messages"`
}
