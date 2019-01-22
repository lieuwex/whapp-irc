package ircConnection

import (
	"fmt"
	"log"
	"time"
)

func LogMessage(time time.Time, from, to, message string) {
	timeStr := time.Format("2006-01-02 15:04:05")
	log.Printf("(%s) %s->%s: %s", timeStr, from, to, message)
}

func FormatPrivateMessage(from, to, line string) string {
	return fmt.Sprintf(":%s PRIVMSG %s :%s", from, to, line)
}
