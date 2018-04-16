package whapp

import "errors"

// ErrLoggedIn will be returned as an error when the called function/method
// expects you to be logged out, but you are logged in.
var ErrLoggedIn = errors.New("logged in, should be logged out")

// ErrLoggedOut will be returned as an error when the called function/method
// expects you to be logged in, but you are logged out.
var ErrLoggedOut = errors.New("logged out, should be logged in")

// ErrCDPUnknown will be returned in some cases as an error when the called
// function/method encountered an unknown error with CDP.
var ErrCDPUnknown = errors.New("unknown CDP error")
