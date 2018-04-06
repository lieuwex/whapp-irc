package main

import (
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/filetype"
	"github.com/mozillazg/go-unidecode"
	"github.com/satori/go.uuid"
)

func strTimestamp() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func getExtension(bytes []byte) string {
	typ, err := filetype.Match(bytes)
	if err != nil {
		return ""
	}

	return typ.Extension
}

func getFileName(bytes []byte) string {
	uid := uuid.NewV4().String()
	fname := strings.Replace(uid, "-", "", -1)

	ext := getExtension(bytes)
	if ext != "" {
		fname += "." + ext
	}

	return fname
}

var unsafeRegex = regexp.MustCompile(`(?i)[^a-z\d+]`)

func IRCsafeString(str string) string {
	str = unidecode.Unidecode(str)
	return unsafeRegex.ReplaceAllLiteralString(str, "")
}

func onInterrupt(fn func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fn()
		os.Exit(1)
	}()
}
