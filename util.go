package main

import (
	"strconv"
	"time"

	"github.com/h2non/filetype"
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
