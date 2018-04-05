package main

import (
	"regexp"
)

const MessageIDListSize = 750

var numberRegex = regexp.MustCompile(`^\+[\d ]+$`)
var nonNumberRegex = regexp.MustCompile(`[^\d]`)

type ContactNames struct {
	Short     string `json:"short"`
	Push      string `json:"push"`
	Formatted string `json:"formatted"`
}

type Contact struct {
	ID      string       `json:"id"`
	Names   ContactNames `json:"names"`
	IsAdmin bool
	IsMe    bool
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

type Chat struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	IsGroupChat  bool      `json:"isGroupChat"`
	Participants []Contact `json:"participants"`

	Joined     bool
	MessageIDs []string
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
