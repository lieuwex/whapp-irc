package ircConnection

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
	"whapp-irc/capabilities"
	"whapp-irc/util"

	"github.com/olebedev/emitter"
	irc "gopkg.in/sorcix/irc.v2"
	tomb "gopkg.in/tomb.v2"
)

const queueSize = 10

type IRCConnection struct {
	Caps *capabilities.CapabilitiesMap

	sendCh    chan string
	receiveCh chan *irc.Message

	tomb    *tomb.Tomb
	emitter *emitter.Emitter

	nick string

	// TODO: remove this
	socket *net.TCPConn
}

func sendMessage(socket *net.TCPConn, msg string) error {
	bytes := []byte(msg + "\n")

	n, err := socket.Write(bytes)
	if err == nil && n != len(bytes) {
		err = fmt.Errorf("bytes length mismatch")
	}
	if err != nil {
		log.Printf("error sending irc message: %s", err)
	}
	return err
}

// HandleConnection wraps around the given socket connection, which you
// shouldn't use after providing it.  It will then handle all the IRC connection
// stuff for you.  You should interface with it using it's methods.
func HandleConnection(ctx context.Context, socket *net.TCPConn) *IRCConnection {
	tomb, ctx := tomb.WithContext(ctx)
	conn := &IRCConnection{
		Caps: capabilities.MakeCapabilitiesMap(),

		sendCh:    make(chan string, queueSize),
		receiveCh: make(chan *irc.Message, queueSize),

		tomb:    tomb,
		emitter: &emitter.Emitter{},

		socket: socket,
	}

	// close socket when connection ends
	go func() {
		<-tomb.Dying()
		socket.Close()
	}()

	// handle sending irc messages out using channel
	tomb.Go(func() error {
		defer close(conn.sendCh)

		for {
			select {
			case <-tomb.Dying():
				return nil

			case msg, ok := <-conn.sendCh:
				if !ok {
					return fmt.Errorf("send channel closed")
				}

				if err := sendMessage(socket, msg); err != nil {
					return err
				}
			}
		}
	})

	// listen for and parse messages.
	// this function also handles IRC commands which are independent of the rest of
	// whapp-irc, such as PINGs.
	tomb.Go(func() error {
		defer close(conn.receiveCh)

		write := conn.WriteNow
		decoder := irc.NewDecoder(bufio.NewReader(socket))
		for {
			msg, err := decoder.Decode()
			if err != nil {
				if err != io.EOF {
					log.Printf("error while listening for IRC messages: %s\n", err)
				}
				return err
			} else if msg == nil {
				log.Println("got invalid IRC message, ignoring")
				continue
			}

			switch msg.Command {
			case "PING":
				str := ":whapp-irc PONG whapp-irc :" + msg.Params[0]
				if err := conn.WriteNow(str); err != nil {
					log.Printf("error while sending PONG: %s", err)
					return err
				}
			case "QUIT":
				log.Printf("received QUIT from %s", conn.nick)
				return fmt.Errorf("got QUIT")

			case "NICK":
				conn.setNick(msg.Params[0])

			case "CAP":
				conn.Caps.StartNegotiation()
				switch msg.Params[0] {
				case "LS":
					write(":whapp-irc CAP * LS :server-time whapp-irc/replay")

				case "LIST":
					caps := conn.Caps.List()
					write(":whapp-irc CAP * LIST :" + strings.Join(caps, " "))

				case "REQ":
					for _, cap := range strings.Split(msg.Trailing(), " ") {
						conn.Caps.Add(cap)
					}
					caps := conn.Caps.List()
					write(":whapp-irc CAP * ACK :" + strings.Join(caps, " "))

				case "END":
					conn.Caps.FinishNegotiation()
				}

			default:
				conn.receiveCh <- msg
			}
		}
	})

	return conn
}

// Write writes the given message with the given timestamp to the connection
func (conn *IRCConnection) Write(time time.Time, msg string) error {
	if conn.Caps.Has("server-time") {
		timeFormat := time.UTC().Format("2006-01-02T15:04:05.000Z")
		msg = fmt.Sprintf("@time=%s %s", timeFormat, msg)
	}

	return sendMessage(conn.socket, msg)
	//conn.ch <- msg
}

// WriteNow writes the given message with a timestamp of now to the connection.
func (conn *IRCConnection) WriteNow(msg string) error {
	return conn.Write(time.Now(), msg)
}

// WriteListNow writes the given messages with a timestamp of now to the
// connection.
func (conn *IRCConnection) WriteListNow(messages []string) error {
	for _, msg := range messages {
		if err := conn.WriteNow(msg); err != nil {
			return err
		}
	}
	return nil
}

// Status writes the given message as if sent by 'status' to the current
// connection.
func (conn *IRCConnection) Status(body string) error {
	util.LogMessage(time.Now(), "status", conn.nick, body)
	msg := FormatPrivateMessage("status", conn.nick, body)
	return conn.WriteNow(msg)
}

// ReceiveChannel returns the channel where new messages are sent on.
func (conn *IRCConnection) ReceiveChannel() <-chan *irc.Message {
	return conn.receiveCh
}

// setNick sets the current connection's nickname to the given new nick, and
// notifies any listeners.
func (conn *IRCConnection) setNick(nick string) {
	conn.nick = nick
	<-conn.emitter.Emit("nick", nick)
}

// NickSetChannel returns a channel that fires when the nickname is changed.
func (conn *IRCConnection) NickSetChannel() <-chan emitter.Event {
	// REVIEW: should this be `On`?
	return conn.emitter.Once("nick")
}

// Nick returns the nickname of the user at the other end of the current
// connection.
func (conn *IRCConnection) Nick() string {
	return conn.nick
}

// Close closes the current connection
func (conn *IRCConnection) Close() {
	conn.tomb.Killf("IRCConnection.Close() called")
}

// StopChannel returns a channel that closes when the current connection is
// being shut down. No messages are sent over this channel.
func (conn *IRCConnection) StopChannel() <-chan struct{} {
	return conn.tomb.Dying()
}
