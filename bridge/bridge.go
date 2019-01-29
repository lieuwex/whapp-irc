package bridge

import (
	"context"
	"log"
	"time"
	"whapp-irc/whapp"

	"github.com/chromedp/chromedp"
)

// A Bridge represents the bridging between an IRC connection and a WhatsApp web
// instance.
type Bridge struct {
	WI *whapp.Instance
}

// Start creates and starts a bridge
func Start(
	ctx context.Context,
	pool *chromedp.Pool,
	loggingLevel whapp.LoggingLevel,
) (bridge *Bridge, err error) {
	wi, err := whapp.MakeInstanceWithPool(ctx, pool, true, loggingLevel)
	if err != nil {
		return nil, err
	}

	// when the context is cancelled, stop the bridge
	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := wi.Shutdown(ctx); err != nil {
			// TODO: how do we handle this?
			log.Printf("error while shutting down: %s", err.Error())
		}

		cancel()
	}()

	return &Bridge{wi}, nil
}
