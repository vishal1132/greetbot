package main

import (
	"log"

	"github.com/vishal1132/greetbot/config"
)

func main() {
	cfg, err := config.GetEnv()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	logger := config.DefaultLogger(cfg)
	if err = runServer(cfg, logger); err != nil {
		logger.Fatal().
			Err(err).
			Msg("failed to run http server")
	}
}
