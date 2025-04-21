package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/alihamzam/git-web-server/internal/git"
	"github.com/alihamzam/git-web-server/pkg"
	"github.com/rs/zerolog/log"
)

type webServer struct {
	mux    *http.ServeMux
	server *http.Server
}

func New(addr string, rwMap *pkg.RWMap, webhooks git.WebhookMap) *webServer {
	log.Info().Msgf("Starting web server on address %s", addr)

	mux := http.NewServeMux()

	server := webServer{
		mux: mux,
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}

	for _, endpoint := range rwMap.GetKeys() {
		log.Debug().Msgf("Creating file endpoint %s", endpoint)
		mux.HandleFunc(endpoint, server.endpointServeFunc(endpoint, rwMap))
	}

	for endpoint, webhookChan := range webhooks {
		log.Debug().Msgf("Creating webhook endpoint %s", endpoint)
		mux.HandleFunc(endpoint, server.webhookServeFunc(webhookChan))
	}

	return &server
}

func (s *webServer) Serve(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		<-ctx.Done()

		if err := s.server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to shutdown server")
		}
	}(ctx)

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("Failed to start server: %w", err)
	}

	return nil
}

func (*webServer) endpointServeFunc(key string, rwMap *pkg.RWMap) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		val, ok := rwMap.Get(key)
		if !ok {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		io.WriteString(w, val)
	}
}

func (*webServer) webhookServeFunc(webhookChan chan struct{}) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		webhookChan <- struct{}{}
		w.WriteHeader(http.StatusOK)

		return
	}
}
