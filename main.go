package main

import (
	"log"
	"net"
	"time"
	"whapp-irc/config"
	"whapp-irc/database"
	"whapp-irc/files"
	"whapp-irc/maps"
	"whapp-irc/whapp"
)

var (
	fs     *files.FileServer
	userDb *database.Database

	loggingLevel      whapp.LoggingLevel
	mapProvider       maps.Provider
	alternativeReplay bool

	startTime = time.Now()
	commit    string
)

func main() {
	config, err := config.ReadEnvVars()
	if err != nil {
		panic(err)
	}
	loggingLevel = config.LoggingLevel
	mapProvider = config.MapProvider
	alternativeReplay = config.AlternativeReplay

	userDb, err = database.MakeDatabase("db/users")
	if err != nil {
		panic(err)
	}

	fs, err = files.MakeFileServer(
		config.FileServerHost,
		config.FileServerPort,
		"files",
		config.FileServerHTTPS,
	)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := fs.Start(); err != nil {
			log.Printf("error while starting fileserver: %s", err)
		}
	}()
	defer fs.Stop()

	addr, err := net.ResolveTCPAddr("tcp", ":"+config.IRCPort)
	if err != nil {
		panic(err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}

	for {
		socket, err := listener.AcceptTCP()
		if err != nil {
			log.Printf("error accepting TCP connection: %s", err)
			continue
		}

		go func() {
			if err := BindSocket(socket); err != nil {
				log.Println(err)
			}
		}()
	}
}
