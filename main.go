package main

import (
	"context"
	"flag"
	"fmt"
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
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if err := Run(sigs); err != nil {
		log.Fatal().Msg(err.Error())
	}

	os.Exit(0)
}

func Run(sigs chan os.Signal) error {
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
		return fmt.Errorf("Failed to parse config: %w", err)
	}

	// wg used to signal successful shutdown
	wg := &sync.WaitGroup{}
	// ctx used to signal intent to shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	rwMap := pkg.NewRWMap()
	git := git.New(rwMap)

	webhooks, err := git.Run(ctx, wg, cfg)
	if err != nil {
		cancel()

		return fmt.Errorf("Failed to obtain initial git data: %w", err)
	}

	webServer := server.New(cfg.Listen, rwMap, webhooks)

	go func() {
		if err := webServer.Serve(ctx, wg); err != nil {
			log.Fatal().Err(err).Msg("Failed to serve")
		}
	}()

	<-sigs

	log.Info().Msg("Shutting down...")

	cancel()
	wg.Wait()

	return nil
}
