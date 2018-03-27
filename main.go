package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

var fs *FileServer

func handleSocket(socket *net.TCPConn) {
	conn, err := MakeConnection()
	if err != nil {
		fmt.Printf("error while making connection: %s\n", err.Error())
	}
	go conn.BindSocket(socket)
}

func listenInternalSockets() {
	addr, err := net.ResolveTCPAddr("tcp", ":1337")
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
			log.Printf("%#v", err)
			continue
		}
		reader := bufio.NewReader(socket)

		l, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		id, _ := strconv.Atoi(strings.TrimSpace(l))

		bridge := Bridges[id]
		if bridge == nil {
			// TODO
			log.Printf("%#v", bridge)
			continue
		}

		go bridge.ProvideSocket(socket)
	}
}

func main() {
	var err error
	fs, err = MakeFileServer("", "3000", "files")
	if err != nil {
		panic(err)
	}
	go fs.Start()

	go listenInternalSockets()

	addr, err := net.ResolveTCPAddr("tcp", ":6060")
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
			log.Printf("%#v", err)
			continue
		}

		go handleSocket(socket)
	}
}
