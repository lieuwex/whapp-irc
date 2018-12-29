package main

import (
	"context"
	"whapp-irc/whapp"
)

type Bridge struct {
	WI *whapp.Instance

	started bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func MakeBridge() *Bridge {
	b := &Bridge{
		started: false,
	}

	onInterrupt(func() {
		b.Stop()
	})

	return b
}

func (b *Bridge) Start() (bool, error) {
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

func (b *Bridge) Stop() bool {
	if !b.started {
		return false
	}

	if b.WI != nil {
		b.WI.Shutdown(b.ctx)
	}
	b.cancel()

	b.started = false
	return true
}

func (b *Bridge) Restart() {
	b.Stop()
	b.Start()
}
