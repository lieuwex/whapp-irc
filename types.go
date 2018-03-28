package main

import (
	"regexp"
	"strings"
	"time"
)

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

	Joined bool
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

type Message struct {
	Timestamp time.Time              `json:"timestamp"`
	Sender    Contact                `json:"sender"`
	Content   string                 `json:"content"`
	Filename  string                 `json:"filename"`
	Caption   string                 `json:"caption"`
	Keys      map[string]interface{} `json:"keys"`
}

func (m *Message) Own(number string) bool {
	return m.Sender.Self(number)
}

type MessageGroup struct {
	Chat     Chat      `json:"chat"`
	Messages []Message `json:"messages"`
}
