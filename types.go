package main

import (
	"regexp"
	"whapp-irc/whapp"
)

const MessageIDListSize = 750

var numberRegex = regexp.MustCompile(`^\+[\d ]+$`)
var nonNumberRegex = regexp.MustCompile(`[^\d]`)

type Participant whapp.Participant

func (p *Participant) FullName() string {
	return p.Contact.FormattedName
}

func (p *Participant) SafeName() string {
	str := p.FullName()
	if numberRegex.MatchString(str) && IRCsafeString(p.Contact.PushName) != "" {
		str = p.Contact.PushName
	}

	return IRCsafeString(str)
}

type Chat struct {
	ID   string
	Name string

	IsGroupChat  bool
	Participants []Participant

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
