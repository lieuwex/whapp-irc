package main

import (
	"regexp"
	"whapp-irc/whapp"
)

const messageIDListSize = 750

var numberRegex = regexp.MustCompile(`^\+[\d ]+$`)
var nonNumberRegex = regexp.MustCompile(`[^\d]`)

type Participant whapp.Participant

func (p *Participant) FullName() string {
	return p.Contact.FormattedName
}

func (p *Participant) SafeName() string {
	str := p.FullName()
	if numberRegex.MatchString(str) && ircSafeString(p.Contact.PushName) != "" {
		str = p.Contact.PushName
	}

	return ircSafeString(str)
}

type Chat struct {
	ID   string
	Name string

	IsGroupChat  bool
	Participants []Participant

	Joined     bool
	MessageIDs []string

	rawChat whapp.Chat
}

func (c *Chat) SafeName() string {
	return ircSafeString(c.Name)
}

func (c *Chat) Identifier() string {
	prefix := ""
	if c.IsGroupChat {
		prefix = "#"
	}

	name := c.SafeName()
	if !c.IsGroupChat && name[0] == '+' {
		name = name[1:]
	}

	return prefix + name
}

func (c *Chat) AddMessageID(id string) {
	if len(c.MessageIDs) >= messageIDListSize {
		c.MessageIDs = c.MessageIDs[1:]
	}
	c.MessageIDs = append(c.MessageIDs, id)
}

func (c *Chat) HasMessageID(id string) bool {
	for _, x := range c.MessageIDs {
		if x == id {
			return true
		}
	}
	return false
}
