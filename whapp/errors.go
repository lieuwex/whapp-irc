package whapp

import (
	"errors"
	"fmt"
)

// ErrLoggedIn will be returned as an error when the called function/method
// expects you to be logged out, but you are logged in.
var ErrLoggedIn = errors.New("logged in, should be logged out")

// ErrLoggedOut will be returned as an error when the called function/method
// expects you to be logged in, but you are logged out.
var ErrLoggedOut = errors.New("logged out, should be logged in")

// ErrCDPUnknown will be returned in some cases as an error when the called
// function/method encountered an unknown error with CDP.
var ErrCDPUnknown = errors.New("unknown CDP error")

// ErrCDP represents an error from CDP.
type ErrCDP struct{ err error }

// errCDP returns a new ErrCDP, wrapping around the given err.
func errCDP(err error) *ErrCDP {
	return &ErrCDP{err}
}

func (err *ErrCDP) Error() string {
	return fmt.Sprintf("CDP error: %s", err.err)
}

// IsErrCDP returns whether or not the given err is an CDP error.
func IsErrCDP(err error) bool {
	switch err.(type) {
	case *ErrCDP:
		return true
	default:
		return false
	}
}
