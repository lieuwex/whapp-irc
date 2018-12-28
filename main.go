package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
	"whapp-irc/database"
	"whapp-irc/files"
	"whapp-irc/maps"
	"whapp-irc/whapp"
)

const (
	defaultHost           = "localhost"
	defaultFileServerPort = "3000"
	defaultIRCPort        = "6060"
	defaultLoggingLevel   = "normal"
	defaultMapProvider    = "google-maps"
	defaultReplayMode     = "normal"
)

var (
	fs           *files.FileServer
	userDb       *database.Database
	loggingLevel whapp.LoggingLevel
	mapProvider  maps.Provider

	startTime         = time.Now()
	alternativeReplay = false
)

func handleSocket(socket *net.TCPConn) error {
	conn, err := MakeConnection()
	if err != nil {
		return fmt.Errorf("error while making connection: %s", err)
	}
	return conn.BindSocket(socket)
}

func readEnvVars() (host, fileServerPort, ircPort, logLevel, mapProvider, replayMode string) {
	host = os.Getenv("HOST")
	if host == "" {
		host = defaultHost
	}

	fileServerPort = os.Getenv("FILE_SERVER_PORT")
	if fileServerPort == "" {
		fileServerPort = defaultFileServerPort
	}

	ircPort = os.Getenv("IRC_SERVER_PORT")
	if ircPort == "" {
		ircPort = defaultIRCPort
	}

	logLevel = os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = defaultLoggingLevel
	}

	mapProvider = os.Getenv("MAP_PROVIDER")
	if mapProvider == "" {
		mapProvider = defaultMapProvider
	}

	replayMode = os.Getenv("REPLAY_MODE")
	if replayMode == "" {
		replayMode = defaultReplayMode
	}

	return
}

func main() {
	host, fileServerPort, ircPort, levelRaw, mapProviderRaw, replayMode := readEnvVars()

	switch strings.ToLower(levelRaw) {
	case "verbose":
		loggingLevel = whapp.LogLevelVerbose
	default:
		loggingLevel = whapp.LogLevelNormal
	}

	switch strings.ToLower(mapProviderRaw) {
	case "openstreetmap", "open-street-map":
		mapProvider = maps.OpenStreetMap
	case "googlemaps", "google-maps":
		mapProvider = maps.GoogleMaps

	default:
		str := fmt.Sprintf("no map provider %s found", mapProviderRaw)
		panic(str)
	}

	alternativeReplay = replayMode == "alternative"

	var err error

	userDb, err = database.MakeDatabase("db/users")
	if err != nil {
		panic(err)
	}

	fs, err = files.MakeFileServer(host, fileServerPort, "files")
	if err != nil {
		panic(err)
	}
	go fs.Start()
	defer fs.Stop()

	addr, err := net.ResolveTCPAddr("tcp", ":"+ircPort)
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
			if err := handleSocket(socket); err != nil {
				log.Println(err)
			}
		}()
	}
}
