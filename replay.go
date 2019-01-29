package main

import (
	"whapp-irc/whapp"
)

func (conn *Connection) hasReplay() bool {
	return conn.irc.Caps.Has("whapp-irc/replay") || alternativeReplay
}

func (conn *Connection) handleWhappMessageReplay(msg whapp.Message) error {
	fn := handlerNormal
	if alternativeReplay {
		fn = handlerAlternativeReplay
	}

	return conn.handleWhappMessage(msg, fn)
}
