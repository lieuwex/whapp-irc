package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
	"whapp-irc/whapp"

	qrcode "github.com/skip2/go-qrcode"
)

// TODO: check if already set-up
func (conn *Connection) setup(ctx context.Context) error {
	bridge, err := StartBridge(ctx)
	if err != nil {
		return err
	}
	conn.bridge = bridge

	// if we have the current user in the database, try to relogin using the
	// previous localStorage state
	var user User
	found, err := userDb.GetItem(conn.irc.Nick(), &user)
	if err != nil {
		return err
	} else if found {
		conn.timestampMap.Swap(user.LastReceivedReceipts)
		conn.chats = user.Chats

		conn.irc.Status("logging in using stored session")

		if err := conn.bridge.WI.Navigate(ctx); err != nil {
			return err
		}
		if err := conn.bridge.WI.SetLocalStorage(
			ctx,
			user.LocalStorage,
		); err != nil {
			log.Printf("error while setting local storage: %s\n", err.Error())
		}
	}

	// open site
	state, err := conn.bridge.WI.Open(ctx)
	if err != nil {
		return err
	}

	// if we aren't logged in yet we have to get the QR code and stuff
	if state == whapp.Loggedout {
		code, err := conn.bridge.WI.GetLoginCode(ctx)
		if err != nil {
			return fmt.Errorf("Error while retrieving login code: %s", err.Error())
		}

		bytes, err := qrcode.Encode(code, qrcode.High, 512)
		if err != nil {
			return err
		}

		timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
		qrFile, err := fs.AddBlob("qr-"+timestamp, "png", bytes)
		if err != nil {
			return err
		}
		defer func() {
			if err = fs.RemoveFile(qrFile); err != nil {
				log.Printf("error while removing QR code: %s\n", err.Error())
			}
		}()

		if err := conn.irc.Status("Scan this QR code: " + qrFile.URL); err != nil {
			return err
		}
	}

	// waiting for login
	if err := conn.bridge.WI.WaitLogin(ctx); err != nil {
		return err
	}
	conn.irc.Status("logged in")

	// get localstorage (that contains new login information), and save it to
	// the database
	conn.localStorage, err = conn.bridge.WI.GetLocalStorage(ctx)
	if err != nil {
		log.Printf("error while getting local storage: %s\n", err.Error())
	} else {
		if err := conn.saveDatabaseEntry(); err != nil {
			return err
		}
	}

	// get information about the user
	conn.me, err = conn.bridge.WI.GetMe(ctx)
	if err != nil {
		return err
	}

	// get raw chats
	rawChats, err := conn.bridge.WI.GetAllChats(ctx)
	if err != nil {
		return err
	}

	// convert chats to internal reprenstation, we do this using a second slice
	// and a WaitGroup to preserve the initial order
	chats := make([]*Chat, len(rawChats))
	var wg sync.WaitGroup
	for i, raw := range rawChats {
		wg.Add(1)
		go func(i int, raw whapp.Chat) {
			defer wg.Done()

			chat, err := conn.convertChat(ctx, raw)
			if err != nil {
				str := fmt.Sprintf("error while converting chat with ID %s, skipping", raw.ID)
				conn.irc.Status(str)
				log.Printf("%s. error: %s", str, err)
				return
			}

			chats[i] = chat
		}(i, raw)
	}
	wg.Wait()

	// add all chats to connection
	for _, chat := range chats {
		if chat == nil {
			// there was an error converting this chat, skip it.
			continue
		}

		conn.addChat(chat)
	}

	return nil
}
