package main

import (
	"context"
	"net"
	"whapp-irc/whapp"
)

type Bridge struct {
	WI *whapp.Instance

	started bool
	ctx     context.Context
	cancel  context.CancelFunc

	socket *net.TCPConn
}

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

func (b *Bridge) Start() (bool, error) {
	if b.started {
		return false, nil
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())

	wi, err := whapp.MakeInstance(b.ctx, chromePath)
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

	b.cancel()

	b.started = false
	return true
}

func (b *Bridge) Restart() {
	b.Stop()
	b.Start()
}
