package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
)

type Bridge struct {
	Chan chan Event

	started bool
	cmd     *exec.Cmd
	outFile *os.File
	id      int

	socket *net.TCPConn
}

var Bridges = make(map[int]*Bridge)

func MakeBridge() *Bridge {
	id := len(Bridges)

	res := &Bridge{
		Chan: make(chan Event),

		started: false,
		id:      id,
	}

	Bridges[id] = res

	return res
}

func (b *Bridge) Start() bool {
	if b.started {
		return false
	}

	b.cmd = exec.Command(
		"/usr/local/bin/python3",
		"./main.py",
		"1337",
		strconv.Itoa(b.id),
	)
	b.outFile, _ = ioutil.TempFile("./", "stdout-main.py-")
	b.cmd.Stdout = b.outFile
	b.cmd.Stderr = b.outFile
	b.cmd.Start()

	b.started = true
	return true
}

func (b *Bridge) Stop() bool {
	if !b.started {
		return false
	}

	// TODO: use context
	b.cmd.Process.Kill()
	b.cmd = nil
	b.outFile.Close()

	b.started = false
	return true
}

func (b *Bridge) ProvideSocket(socket *net.TCPConn) {
	b.socket = socket
	reader := bufio.NewReader(socket)

	for {
		bytes, err := reader.ReadBytes('\n')
		if err != nil {
			fmt.Println("connection broken, restarting main.py")
			b.Stop()
			b.Start()
			return
		}

		var event Event
		if err := json.Unmarshal(bytes, &event); err != nil {
			fmt.Printf("briodge json err %#v\n", err)
			continue
		}

		b.Chan <- event
	}
}

func (b *Bridge) Write(cmd Command) error {
	if b.socket == nil {
		return fmt.Errorf("socket nil")
	}

	bytes, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	fmt.Printf("writing '%s'\n", string(bytes))
	bytes = append(bytes, '\n')

	n, err := b.socket.Write(bytes)
	if err != nil {
		return err
	} else if n != len(bytes) {
		return fmt.Errorf("bytes length mismatch")
	}

	return nil
}
