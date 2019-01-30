package util

import (
	"log"
	"mime"
	"time"

	"github.com/h2non/filetype"
)

func getExtension(bytes []byte) string {
	typ, err := filetype.Match(bytes)
	if err != nil {
		return ""
	}

	res := typ.Extension
	if res == "unknown" {
		return ""
	}
	return res
}

func getExtensionByMime(typ string) (string, error) {
	extensions, err := mime.ExtensionsByType(typ)
	if err != nil {
		return "", err
	}

	if len(extensions) == 0 {
		return "", nil
	}

	return extensions[0][1:], nil
}

func GetExtensionByMimeOrBytes(mime string, bytes []byte) string {
	if res, err := getExtensionByMime(mime); res != "" && err == nil {
		return res
	}

	return getExtension(bytes)
}

func Plural(count int, singular, plural string) string {
	if count == 1 || count == -1 {
		return singular
	}

	return plural
}

func LogMessage(time time.Time, from, to, message string) {
	timeStr := time.Format("2006-01-02 15:04:05")
	log.Printf("(%s) %s->%s: %s", timeStr, from, to, message)
}

// LogIfErr logs the given err with the given prefix if err is not nil.
func LogIfErr(prefix string, err error) {
	if err != nil {
		log.Printf("%s: %s", prefix, err)
	}
}
