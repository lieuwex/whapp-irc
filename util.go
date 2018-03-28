package main

import (
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

func getFileName(bytes []byte) string {
	ext := getExtension(bytes)

	ts := strTimestamp()
	if ext != "" {
		ts += "." + ext
	}
	return ts
}

var unsafeRegex = regexp.MustCompile(`(?i)[^a-z\d+]`)

func IRCsafeString(str string) string {
	str = unidecode.Unidecode(str)
	return unsafeRegex.ReplaceAllLiteralString(str, "")
}
