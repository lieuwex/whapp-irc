package util

import (
	"log"
	"mime"
	"time"

	"github.com/h2non/filetype"
)

// getExtensionByBytes returns the extension by the given bytes.
func getExtensionByBytes(bytes []byte) string {
	typ, err := filetype.Match(bytes)
	if err != nil || typ == filetype.Unknown {
		return ""
	}
	return typ.Extension
}

// getExtensionByMime returns the extension by the given mime type.
func getExtensionByMime(typ string) (string, error) {
	extensions, err := mime.ExtensionsByType(typ)
	if err != nil {
		return "", err
	} else if len(extensions) == 0 {
		return "", nil
	}
	return extensions[0][1:], nil
}

// GetExtensionByMimeOrBytes returns the extension by the given mime, or if that
// fails, by the given bytes.
func GetExtensionByMimeOrBytes(mime string, bytes []byte) string {
	if res, err := getExtensionByMime(mime); res != "" && err == nil {
		return res
	}

	return getExtensionByBytes(bytes)
}

// Plural returns singular if count is ±1, plural otherwise.
func Plural(count int, singular, plural string) string {
	if count == 1 || count == -1 {
		return singular
	}

	return plural
}

// LogMessage logs the given chat message to the log.
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
