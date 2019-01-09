package main

import (
	"context"
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
	b := &Bridge{
		started: false,
	}

	onInterrupt(func() {
		if b.WI != nil {
			b.WI.Shutdown(b.ctx)
		}

		if b.cancel != nil {
			b.cancel()
		}
	})

	return b
}

// Start starts the current bridge instance.
func (b *Bridge) Start() (started bool, err error) {
	if b.started {
		return false, nil
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())

	wi, err := whapp.MakeInstance(b.ctx, chromePath, true, loggingLevel)
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

	b.started = false
	return true
}

// Restart restarts the current bridge instance.
func (b *Bridge) Restart() {
	b.Stop()
	b.Start()
}
