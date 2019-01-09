package main

import (
	"context"
	"whapp-irc/whapp"
)

// MessageRes represents a failure or success receiving a WhatsApp message.
type MessageRes struct {
	Err     error
	Message whapp.Message
}

// MessageQueue is a queue containing "futures" to MessageRes instances.
type MessageQueue chan chan MessageRes

// GetMessageQueue wraps around the given WhatsApp message channel and makes a
// queue, queueing a maximum of queueSize items.
func GetMessageQueue(ctx context.Context, ch <-chan whapp.Message, queueSize int) MessageQueue {
	queue := make(MessageQueue, queueSize)

	go func() {
		defer close(queue)

		for {
			select {
			case <-ctx.Done():
				return

			case msg := <-ch:
				ch := make(chan MessageRes)
				queue <- ch

				go func() {
					err := downloadAndStoreMedia(msg)
					ch <- MessageRes{
						Err:     err,
						Message: msg,
					}
					close(ch)
				}()
			}
		}
	}()

	return queue
}
