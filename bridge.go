package main

import (
	"context"
	"time"
	"whapp-irc/whapp"
)

// A Bridge represents the bridging between an IRC connection and a WhatsApp web
// instance.
type Bridge struct {
	WI *whapp.Instance

	started bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// MakeBridge makes and returns a new Bridge instance.
func MakeBridge() *Bridge {
	return &Bridge{
		started: false,
	}
}

// Start starts the current bridge instance.
func (b *Bridge) Start() (started bool, err error) {
	if b.started {
		return false, nil
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())

	wi, err := whapp.MakeInstance(b.ctx, true, loggingLevel)
	if err != nil {
		return false, err
	}

	b.started = true
	b.WI = wi

	return true, nil
}

// Stop stops the current bridge instance.
func (b *Bridge) Stop() (stopped bool) {
	if !b.started {
		return false
	}

	b.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	b.cancel = cancel
	if err := b.WI.Shutdown(ctx); err != nil {
		// TODO: how do we handle this?
		println("error while shutting down: " + err.Error())
	}

	b.started = false
	return true
}
