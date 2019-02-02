package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
	"whapp-irc/bridge"
	"whapp-irc/ircConnection"
	"whapp-irc/timestampMap"
	"whapp-irc/types"
	"whapp-irc/util"
	"whapp-irc/whapp"

	qrcode "github.com/skip2/go-qrcode"
)

func setupConnection(ctx context.Context, irc *ircConnection.Connection) (*Connection, error) {
	wi, err := bridge.Start(ctx, pool, conf.LogLevel)
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		WI:    wi,
		Chats: &types.ChatList{},

		irc: irc,

		timestampMap: timestampMap.New(),
	}

	// if we have the current user in the database, try to relogin using the
	// previous localStorage state
	var user types.User
	found, err := userDb.GetItem(conn.irc.Nick(), &user)
	if err != nil {
		return nil, err
	} else if found {
		conn.timestampMap.Swap(user.LastReceivedReceipts)
		conn.Chats = types.ChatListFromSlice(user.Chats)

		conn.irc.Status("logging in using stored session")

		if err := wi.Navigate(ctx); err != nil {
			return nil, err
		}
		if err := wi.SetLocalStorage(
			ctx,
			user.LocalStorage,
		); err != nil {
			log.Printf("error while setting local storage: %s\n", err)
		}
	}

	// open site
	state, err := wi.Open(ctx)
	if err != nil {
		return nil, err
	}

	// if we aren't logged in yet we have to get the QR code and stuff
	if state == whapp.Loggedout {
		code, err := wi.GetLoginCode(ctx)
		if err != nil {
			return nil, fmt.Errorf("Error while retrieving login code: %s", err)
		}

		bytes, err := qrcode.Encode(code, qrcode.High, 512)
		if err != nil {
			return nil, err
		}

		timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
		qrFile, err := fs.AddBlob("qr-"+timestamp, "png", bytes)
		if err != nil {
			return nil, err
		}
		defer func() {
			err := fs.RemoveFile(qrFile)
			util.LogIfErr("error while removing QR code", err)
		}()

		if err := conn.irc.Status("Scan this QR code: " + qrFile.URL); err != nil {
			return nil, err
		}
	}

	// waiting for login
	if err := wi.WaitLogin(ctx); err != nil {
		return nil, err
	}
	conn.irc.Status("logged in")

	// get localstorage (that contains new login information), and save it to
	// the database
	conn.localStorage, err = wi.GetLocalStorage(ctx)
	if err != nil {
		log.Printf("error while getting local storage: %s\n", err)
	} else {
		if err := conn.saveDatabaseEntry(); err != nil {
			return nil, err
		}
	}

	// get information about the user
	conn.me, err = wi.GetMe(ctx)
	if err != nil {
		return nil, err
	}

	// get raw chats
	rawChats, err := wi.GetAllChats(ctx)
	if err != nil {
		return nil, err
	}

	// convert chats to internal reprenstation, we do this using a second slice
	// and a WaitGroup to preserve the initial order
	chats := make([]*types.Chat, len(rawChats))
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

	return conn, nil
}
