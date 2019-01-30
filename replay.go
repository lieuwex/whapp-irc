package main

import (
	"context"
	"whapp-irc/whapp"
)

func (conn *Connection) hasReplay() bool {
	return conn.irc.Caps.Has("whapp-irc/replay") || conf.AlternativeReplay
}

func (conn *Connection) handleWhappMessageReplay(ctx context.Context, msg whapp.Message) error {
	fn := handlerNormal
	if conf.AlternativeReplay {
		fn = handlerAlternativeReplay
	}

	return conn.handleWhappMessage(ctx, msg, fn)
}
