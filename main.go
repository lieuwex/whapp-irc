package main

import (
	"log"
	"net"
	"os"
	"strings"
	"whapp-irc/database"
	"whapp-irc/files"
	"whapp-irc/whapp"
)

const (
	defaultHost           = "localhost"
	defaultFileServerPort = "3000"
	defaultIRCPort        = "6060"
	defaultLoggingLevel   = "normal"
)

var fs *files.FileServer
var userDb *database.Database
var loggingLevel whapp.LoggingLevel

func handleSocket(socket *net.TCPConn) {
	conn, err := MakeConnection()
	if err != nil {
		log.Printf("error while making connection: %s", err)
	}
	go conn.BindSocket(socket)
}

func parseEnvVars() (host, fileServerPort, ircPort, logLevel string) {
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

	return
}

func main() {
	host, fileServerPort, ircPort, levelRaw := parseEnvVars()

	switch strings.ToLower(levelRaw) {
	case "verbose":
		loggingLevel = whapp.LogLevelVerbose
	default:
		loggingLevel = whapp.LogLevelNormal
	}

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
			log.Printf("error accepting TCP connection: %#v", err)
			continue
		}

		go handleSocket(socket)
	}
}
