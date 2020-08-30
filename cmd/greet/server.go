package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/vishal1132/greetbot/config"
)

func runServer(cfg config.Config, logger zerolog.Logger) error {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)
	logger.Info().
		Str("env", string(cfg.Env)).
		Str("app", cfg.Heroku.AppName).
		Str("dyno_id", cfg.Heroku.DynoID).
		Str("commit", cfg.Heroku.Commit).
		Str("slack_client_id", cfg.Slack.ClientID).
		Str("log_level", cfg.LogLevel.String()).
		Msg("configuration values")

	_, cancel := context.WithCancel(context.Background())

	defer cancel()

	// sc := slack.New(cfg.Slack.BotAccessToken, slack.OptionHTTPClient(newHTTPClient()))

	// signal handling / graceful shutdown goroutine
	go func() {
		sig := <-signalCh

		cancel()

		logger.Info().
			Str("signal", sig.String()).
			Msg("shutting down http server gracefully")
	}()

	// set up the handler
	hnd := handler{
		l: &logger,
	}

	mux := http.NewServeMux()

	slackHandler := chMiddlewareFactory(
		logger,
		slackSignatureMiddlewareFactory(
			cfg.Slack.RequestSecret, cfg.Slack.RequestToken, cfg.Slack.AppID, cfg.Slack.TeamID, &logger, hnd.handleSlackEvent,
		),
	)

	mux.HandleFunc("/_ruok", hnd.handleRUOK)
	mux.HandleFunc("/slack/event", slackHandler)

	socketAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	logger.Info().
		Str("addr", socketAddr).
		Msg("binding to TCP socket")

	listener, err := net.Listen("tcp", socketAddr)
	if err != nil {
		return fmt.Errorf("failed to open HTTP socket: %w", err)
	}

	defer func() { _ = listener.Close() }()

	// set up the HTTP server
	httpSrvr := &http.Server{
		Handler:     mux,
		ReadTimeout: 20 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	serveStop, serverShutdown := make(chan struct{}), make(chan struct{})
	var serveErr, shutdownErr error

	// HTTP server parent goroutine
	go func() {
		defer close(serveStop)
		serveErr = httpSrvr.Serve(listener)
	}()

	// signal handling / graceful shutdown goroutine
	go func() {
		defer close(serverShutdown)
		sig := <-signalCh

		logger.Info().
			Str("signal", sig.String()).
			Msg("shutting HTTP server down gracefully")

		cctx, ccancel := context.WithTimeout(context.Background(), 25*time.Second)

		defer ccancel()
		defer cancel()

		if shutdownErr = httpSrvr.Shutdown(cctx); shutdownErr != nil {
			logger.Error().
				Err(shutdownErr).
				Msg("failed to gracefully shut down HTTP server")
		}
	}()

	// wait for it to die
	<-serverShutdown
	<-serveStop

	// log errors for informational purposes
	logger.Info().
		AnErr("serve_err", serveErr).
		AnErr("shutdown_err", shutdownErr).
		Msg("server shut down")

	return nil
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: newHTTPTransport(),
	}
}

// newHTTPTransport returns an *http.Transport with some reasonable defaults.
func newHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
	}
}
