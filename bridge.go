package main

import (
	"context"
	"net"
	"whapp-irc/whapp"
)

type Bridge struct {
	WI *whapp.WhappInstance

	started bool
	ctx     context.Context
	cancel  context.CancelFunc

	socket *net.TCPConn
}

func MakeBridge() *Bridge {
	ctx, cancel := context.WithCancel(context.Background())

	res := &Bridge{
		started: false,
		ctx:     ctx,
		cancel:  cancel,
	}

	return res
}

func (b *Bridge) Start() bool {
	if b.started {
		return false
	}

	wi, err := whapp.MakeWhappInstance(b.ctx)

	b.started = true
	if err != nil {
		println("error while making instance")
		b.Restart()
	}

	b.WI = wi
	return true
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
