package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"whapp-irc/maps"
	"whapp-irc/whapp"
)

// Config contains all the possible configuration options and their values
type Config struct {
	FileServerHost  string
	FileServerPort  string
	FileServerHTTPS bool

	IRCPort string

	LoggingLevel whapp.LoggingLevel

	MapProvider maps.Provider

	AlternativeReplay bool
}

func getEnvDefault(env, def string) string {
	res := os.Getenv(env)
	if res == "" {
		return def
	}
	return res
}

// ReadEnvVars reads environment variables and returns a Config instance
// containing the parsed values, or an error.
func ReadEnvVars() (Config, error) {
	host := getEnvDefault("HOST", "localhost")
	fileServerPort := getEnvDefault("FILE_SERVER_PORT", "3000")
	fileServerUseHTTPS := getEnvDefault("FILE_SERVER_HTTPS", "false")
	ircPort := getEnvDefault("IRC_SERVER_PORT", "6060")
	logLevelRaw := getEnvDefault("LOG_LEVEL", "normal")
	mapProviderRaw := getEnvDefault("MAP_PROVIDER", "google-maps")
	replayMode := getEnvDefault("REPLAY_MODE", "normal")

	useHTTPS, err := strconv.ParseBool(fileServerUseHTTPS)
	if err != nil {
		return Config{}, err
	}

	var logLevel whapp.LoggingLevel
	switch strings.ToLower(logLevelRaw) {
	case "verbose":
		logLevel = whapp.LogLevelVerbose
	case "normal":
		logLevel = whapp.LogLevelNormal

	default:
		err := fmt.Errorf("no log level %s found", logLevelRaw)
		return Config{}, err
	}

	var mapProvider maps.Provider
	switch strings.ToLower(mapProviderRaw) {
	case "openstreetmap", "open-street-map":
		mapProvider = maps.OpenStreetMap
	case "googlemaps", "google-maps":
		mapProvider = maps.GoogleMaps

	default:
		err := fmt.Errorf("no map provider %s found", mapProviderRaw)
		return Config{}, err
	}

	return Config{
		FileServerHost:  host,
		FileServerPort:  fileServerPort,
		FileServerHTTPS: useHTTPS,

		IRCPort: ircPort,

		LoggingLevel: logLevel,

		MapProvider: mapProvider,

		AlternativeReplay: replayMode == "alternative",
	}, nil
}
