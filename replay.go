package main

import "whapp-irc/whapp"

func (conn *Connection) hasReplay() bool {
	return conn.irc.Caps.Has("whapp-irc/replay") || alternativeReplay
}

func (conn *Connection) handleWhappMessageReplay(msg whapp.Message) error {
	if alternativeReplay {
		return conn.alternativeReplayWhappMessageHandle(msg)
	}

	return conn.handleWhappMessage(msg)
}
