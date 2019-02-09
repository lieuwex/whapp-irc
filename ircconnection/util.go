package ircconnection

import (
	"encoding/hex"
	"fmt"
	"regexp"

	unidecode "github.com/mozillazg/go-unidecode"
	"github.com/wangii/emoji"
)

// formatPrivateMessage formats the given line for a private message.
func formatPrivateMessage(from, to, line string) string {
	return fmt.Sprintf(":%s PRIVMSG %s :%s", from, to, line)
}

var unsafeRegex = regexp.MustCompile(`(?i)[^a-z\d+:]`)

// SafeString converts emojis into their corresponding tag, converts Unicode
// into their matching ASCII representation and removes and left non safe
// characters in the given str.
func SafeString(str string) string {
	emojiTagged := emoji.UnicodeToEmojiTag(str)
	decoded := unidecode.Unidecode(emojiTagged)
	ircSafe := unsafeRegex.ReplaceAllLiteralString(decoded, "")

	if ircSafe == "" {
		return "x" + hex.EncodeToString([]byte(str))
	}
	return ircSafe
}
