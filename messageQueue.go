package main

import (
	"context"
	"path/filepath"
	"whapp-irc/whapp"
)

func handleMessage(msg whapp.Message) error {
	if _, ok := fs.HashToPath[msg.MediaFileHash]; msg.IsMMS && !ok {
		bytes, err := msg.DownloadMedia()
		if err != nil {
			return err
		}

		ext := getExtensionByMimeOrBytes(msg.MimeType, bytes)
		if ext == "" {
			ext = filepath.Ext(msg.MediaFilename)
			if ext != "" {
				ext = ext[1:]
			}
		}

		if _, err := fs.AddBlob(
			msg.MediaFileHash,
			ext,
			bytes,
		); err != nil {
			return err
		}
	}

	return nil
}

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
					err := handleMessage(msg)
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
