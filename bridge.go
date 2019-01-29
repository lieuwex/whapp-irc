package main

import (
	"context"
	"log"
	"time"
	"whapp-irc/whapp"
)

// A Bridge represents the bridging between an IRC connection and a WhatsApp web
// instance.
type Bridge struct {
	WI  *whapp.Instance
	ctx context.Context
}

// MakeBridge makes and returns a new Bridge instance.
func MakeBridge() *Bridge {
	return &Bridge{}
}

// Start starts the current bridge instance.
func (b *Bridge) Start(ctx context.Context) (started bool, err error) {
	if b.WI != nil {
		return false, nil
	}

	ctx, cancel := context.WithCancel(ctx)

	wi, err := whapp.MakeInstanceWithPool(ctx, pool, true, loggingLevel)
	if err != nil {
		cancel()
		return false, err
	}

	b.ctx = ctx
	b.WI = wi

	// when the context is cancelled, stop the bridge
	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := b.WI.Shutdown(ctx); err != nil {
			// TODO: how do we handle this?
			log.Printf("error while shutting down: %s", err.Error())
		}

		cancel()

		b.ctx = nil
		b.WI = nil
	}()

	return true, nil
}
