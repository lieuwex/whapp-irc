package main

import (
	"regexp"
	"whapp-irc/whapp"
)

const MessageIDListSize = 750

var numberRegex = regexp.MustCompile(`^\+[\d ]+$`)
var nonNumberRegex = regexp.MustCompile(`[^\d]`)

type Contact struct {
	WhappContact whapp.Contact
	IsAdmin      bool
}

func (c *Contact) FullName() string {
	return c.WhappContact.FormattedName
}

func (c *Contact) SafeName() string {
	str := c.FullName()
	if numberRegex.MatchString(str) && IRCsafeString(c.WhappContact.PushName) != "" {
		str = c.WhappContact.PushName
	}

	return IRCsafeString(str)
}

type Chat struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	IsGroupChat  bool      `json:"isGroupChat"`
	Participants []Contact `json:"participants"`

	Joined     bool
	MessageIDs []string

	rawChat *whapp.Chat
}

func (c *Chat) SafeName() string {
	return IRCsafeString(c.Name)
}

func (c *Chat) Identifier() string {
	prefix := ""
	if c.IsGroupChat {
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
