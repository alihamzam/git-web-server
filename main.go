package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog/log"
)

var (
	version, commitHash, buildTime string //nolint:gochecknoglobals
)

func main() {
	cfgFile := flag.String("config", "config.yaml", "Path to config.yaml file")
	flag.Parse()
	if cfgFile == nil || *cfgFile == "" {
		log.Fatal().Msg("Invalid config file provided")
	}

	cfg, err := Parse(*cfgFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config")
	}

	endpoints, err := Obtain(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to obtain initial git data")
	}

	if err := Server(":1234", endpoints); err != nil {
		log.Fatal().Err(err).Msg("Failed to serve")
	}

	os.Exit(0)
}
