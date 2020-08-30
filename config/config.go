// Package config provides the configuration helpers for gopher, for pulling
// configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// Environment is the current runtime environment.
type Environment string

const (
	// Development is for when it's the development environment
	Development Environment = "development"

	// Testing is WISOTT
	Testing Environment = "testing"

	// Staging is WISOTT
	Staging Environment = "staging"

	// Production is WISOTT
	Production Environment = "production"
)

func strToEnv(s string) Environment {
	switch strings.ToLower(s) {
	case "production":
		return Production
	case "staging":
		return Staging
	case "testing":
		return Testing
	default:
		return Development
	}
}

// Hero is the Heroku environment configuration
type Hero struct {
	// AppID is the HEROKU_APP_ID
	AppID string

	// AppName is the HEROKU_APP_NAME
	AppName string

	// DynoID is the HEROKU_DYNO_ID
	DynoID string

	// Commit is the HEROKU_SLUG_COMMIT
	Commit string
}

// S is the Slack environment configuration
type S struct {
	// AppID is the Slack App ID
	// Env: SLACK_APP_ID
	AppID string

	// TeamID is the workspace the app is deployed to
	// ENV: SLACK_TEAM_ID
	TeamID string

	// BotAccessToken is the bot access token for API calls
	// ENV: SLACK_BOT_ACCESS_TOKEN
	BotAccessToken string

	// ClientID is the Client ID
	// Env: SLACK_CLIENT_ID
	ClientID string

	// ClientSecret is the Client secret
	// Env: SLACK_CLIENT_SECRET
	ClientSecret string

	// RequestSecret is the HMAC signing secret used for Slack request signing
	// Env: SLACK_REQUEST_SECRET
	RequestSecret string

	// RequestToken is the Slack verification token
	// Env: SLACK_REQUEST_TOKEN
	RequestToken string
}

// Config is the configuration struct.
type Config struct {
	// LogLevel is the logging level
	// Env: LOG_LEVEL
	LogLevel zerolog.Level

	// Env is the current environment.
	// Env: ENV
	Env Environment

	// Port is the TCP port for web workers to listen on, loaded from PORT
	// Env: PORT
	Port uint16

	// Heroku are the Labs Dyno Metadata environment variables
	Heroku Hero

	// Slack is the Slack configuration, loaded from a few SLACK_* environment
	// variables
	Slack S
}

// GetEnv function loads the environment variables
func GetEnv() (Config, error) {
	var config Config
	if p := os.Getenv("PORT"); len(p) > 0 {
		u, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return Config{}, fmt.Errorf("failed to parse PORT: %w", err)
		}

		config.Port = uint16(u)
	}
	ll := os.Getenv("GreetBotLogLevel")
	if len(ll) == 0 {
		ll = "info"
	}
	l, err := zerolog.ParseLevel(ll)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse GOPHER_LOG_LEVEL: %w", err)
	}
	config.LogLevel = l
	config.Env = strToEnv(os.Getenv("ENV"))
	config.Heroku.AppID = os.Getenv("HEROKU_APP_ID")
	config.Heroku.AppName = os.Getenv("HEROKU_APP_NAME")
	config.Heroku.DynoID = os.Getenv("HEROKU_DYNO_ID")
	config.Heroku.Commit = os.Getenv("HEROKU_SLUG_COMMIT")

	config.Slack.AppID = os.Getenv("ANSH_SLACK_APP_ID")
	config.Slack.TeamID = os.Getenv("ANSH_SLACK_TEAM_ID")
	config.Slack.ClientID = os.Getenv("ANSH_SLACK_CLIENT_ID")
	config.Slack.RequestToken = os.Getenv("ANSH_SLACK_REQUEST_TOKEN")

	config.Slack.ClientSecret = os.Getenv("ANSH_SLACK_CLIENT_SECRET")
	config.Slack.RequestSecret = os.Getenv("ANSH_SLACK_REQUEST_SECRET")
	config.Slack.BotAccessToken = os.Getenv("ANSH_SLACK_BOT_ACCESS_TOKEN")

	_ = os.Unsetenv("ANSH_SLACK_CLIENT_SECRET")    // paranoia
	_ = os.Unsetenv("ANSH_SLACK_REQUEST_SECRET")   // paranoia
	_ = os.Unsetenv("ANSH_SLACK_BOT_ACCESS_TOKEN") // paranoia

	return config, nil
}

// DefaultLogger returns a zerolog.Logger using settings from our config struct.
func DefaultLogger(config Config) zerolog.Logger {
	// set up zerolog
	zerolog.TimestampFieldName = "timestamp"
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(config.LogLevel)

	// set up logging
	return zerolog.New(os.Stdout).
		With().Timestamp().Logger()
}
