package main

import (
	"encoding/base64"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"time"

	"github.com/h2non/filetype"
	"github.com/mozillazg/go-unidecode"
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

func b64tob64url(str string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func b64urltob64(str string) (string, error) {
	bytes, err := base64.RawURLEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}
