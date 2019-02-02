package types

import (
	"regexp"
	"whapp-irc/ircConnection"
	"whapp-irc/whapp"
)

const messageIDListSize = 750

var (
	numberRegex    = regexp.MustCompile(`^\+[\d ]+$`)
	nonNumberRegex = regexp.MustCompile(`[^\d]`)
)

// A Participant is an user on WhatsApp.
type Participant whapp.Participant

// FullName returns the full/formatted name for the current Participant.
func (p *Participant) FullName() string {
	return p.Contact.FormattedName
}

// SafeName returns the irc-safe name for the current Participant.
func (p *Participant) SafeName() string {
	str := p.FullName()

	if numberRegex.MatchString(str) {
		if res := ircConnection.SafeString(p.Contact.PushName); res != "" {
			return res
		}
	}

	return ircConnection.SafeString(str)
}

// Chat represents a chat on the bridge.
// It can be private or public, and is always accompanied by a WhatsApp chat.
type Chat struct {
	ID   whapp.ID
	Name string

	IsGroupChat  bool
	Participants []Participant

	Joined     bool
	MessageIDs []string

	RawChat whapp.Chat
}

// SafeName returns the IRC-safe name for the current chat.
func (c *Chat) SafeName() string {
	return ircConnection.SafeString(c.Name)
}

// Identifier returns the safe IRC identifier for the current chat.
func (c *Chat) Identifier() string {
	prefix := ""
	if c.IsGroupChat {
		prefix = "#"
	}

	name := c.SafeName()
	if !c.IsGroupChat && len(name) > 0 && name[0] == '+' {
		name = name[1:]
	}

	return prefix + name
}

// AddMessageID adds the given id to the chat, so that it's known as
// received/sent.
func (c *Chat) AddMessageID(id string) {
	if len(c.MessageIDs) >= messageIDListSize {
		c.MessageIDs = c.MessageIDs[1:]
	}
	c.MessageIDs = append(c.MessageIDs, id)
}

// HasMessageID returns whether or not a message with the given id has been
// received/sent in the current chat.
func (c *Chat) HasMessageID(id string) bool {
	for _, x := range c.MessageIDs {
		if x == id {
			return true
		}
	}
	return false
}

// User represents the on-disk format of an user of the bridge.
type User struct {
	LocalStorage         map[string]string `json:"localStorage"`
	LastReceivedReceipts map[string]int64  `json:"lastReceivedReceipts"`
	Chats                []ChatListItem    `json:"chats"`
}
