package database

import "errors"

// ErrIDEmpty will be returned as an error when an empty ID has been given.
var ErrIDEmpty = errors.New("ID can't be empty")
