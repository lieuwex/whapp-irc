package whapp

// LoginState represents the login state of an Instance.
type LoginState int

const (
	// Loggedout is the state of a logged out instance.
	Loggedout LoginState = iota
	// Loggedin is the state of a logged in instance.
	Loggedin = iota
)

// LoggingLevel represents the level of logging by the CDP instance.
type LoggingLevel int

const (
	// LogLevelVerbose is the highest level of logging verbosity.
	LogLevelVerbose LoggingLevel = iota
	// LogLevelNormal is the normal level of logging verbosity.
	LogLevelNormal = iota
)
