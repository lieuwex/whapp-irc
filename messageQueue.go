package main

import (
	"context"
	"whapp-irc/whapp"
)

type MessageRes struct {
	Err     error
	Message whapp.Message
}

type MessageQueue chan chan MessageRes

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
