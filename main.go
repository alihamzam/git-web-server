package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alihamzam/git-web-server/config"
	"github.com/alihamzam/git-web-server/internal/git"
	"github.com/alihamzam/git-web-server/internal/server"
	"github.com/alihamzam/git-web-server/pkg"
	zlog "github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version, commitHash, buildTime string //nolint:gochecknoglobals
)

func main() {
	zlog.SetGlobalLevel(zlog.InfoLevel)

	log.Info().
		Str("version", version).
		Str("commit_hash", commitHash).
		Str("build_time", buildTime).
		Msg("Start git web server")

	cfgFile := flag.String("config", "config.yaml", "Path to config.yaml file")
	debug := flag.Bool("debug", false, "Path to config.yaml file")
	flag.Parse()
	if debug != nil && *debug {
		zlog.SetGlobalLevel(zlog.DebugLevel)
	}

	cfg, err := config.Parse(cfgFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config")
	}

	// ctx used to signal intent to shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// wg used to signal successful shutdown
	wg := &sync.WaitGroup{}

	rwMap := pkg.NewRWMap()
	git := git.New(rwMap)

	webhooks, err := git.Run(ctx, wg, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to obtain initial git data")
	}

	webServer := server.New(cfg.Listen, rwMap, webhooks)

	go func() {
		if err := webServer.Serve(ctx, wg); err != nil {
			log.Fatal().Err(err).Msg("Failed to serve")
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	log.Info().Msg("Shutting down...")

	cancel()
	wg.Wait()

	os.Exit(0)
}
