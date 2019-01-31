package files

import "errors"

// ErrHashEmpty is returned when the given hash is empty.
var ErrHashEmpty = errors.New("hash is empty")

// ErrBytesEmpty is returned when the given byte slice is empty.
var ErrBytesEmpty = errors.New("bytes are empty")
