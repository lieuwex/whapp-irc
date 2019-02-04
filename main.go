package main

import (
	"log"
	"net"
	"os"
	"runtime/pprof"
	"time"
	"whapp-irc/config"
	"whapp-irc/database"
	"whapp-irc/files"
	"whapp-irc/whapp"

	"github.com/chromedp/chromedp"
)

var (
	conf config.Config

	fs     *files.FileServer
	userDb *database.Database
	pool   *chromedp.Pool

	startTime = time.Now()
	commit    string
)

func main() {
	f, err := os.Create("cpu.profile")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	conf, err = config.ReadEnvVars()
	if err != nil {
		panic(err)
	}

	userDb, err = database.MakeDatabase("db/users")
	if err != nil {
		panic(err)
	}

	fs, err = files.MakeFileServer(
		conf.FileServerHost,
		conf.FileServerPort,
		"files",
		conf.FileServerHTTPS,
	)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := fs.Serve(); err != nil {
			log.Fatalf("error while serving fileserver: %s", err)
		}
	}()

	pool, err = func() (*chromedp.Pool, error) {
		switch conf.LogLevel {
		case whapp.LogLevelVerbose:
			return chromedp.NewPool(chromedp.PoolLog(log.Printf, log.Printf, log.Printf))
		default:
			return chromedp.NewPool()
		}
	}()
	if err != nil {
		panic(err)
	}
	//defer pool.Shutdown()

	addr, err := net.ResolveTCPAddr("tcp", ":"+conf.IRCPort)
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

		if err := BindSocket(socket); err != nil {
			log.Println(err)
		}

		return
	}
}
