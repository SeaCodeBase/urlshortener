// backend/cmd/server/shutdown.go
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// shutdownTimeout bounds how long the HTTP servers get to drain in-flight
// requests. docker-compose's stop_grace_period (30s) leaves room for the
// worker stops that follow.
const shutdownTimeout = 20 * time.Second

// namedServer is an HTTP server reduced to what runServers needs, so the
// orchestration is testable without sockets.
type namedServer struct {
	name     string
	listen   func() error                    // blocks; http.ErrServerClosed after shutdown
	shutdown func(ctx context.Context) error // graceful; must honour ctx
}

// namedStop is a synchronous worker stop.
type namedStop struct {
	name string
	stop func()
}

func httpServer(name string, srv *http.Server) namedServer {
	return namedServer{name: name, listen: srv.ListenAndServe, shutdown: srv.Shutdown}
}

// runServers starts every server, waits until ctx is done or a server fails
// to listen, then shuts all servers down concurrently (bounded by timeout)
// and finally runs the stops in order. It returns the first listen error,
// or nil when shutdown was requested through ctx.
func runServers(ctx context.Context, servers []namedServer, stops []namedStop, timeout time.Duration) error {
	logCtx := context.Background() // ctx is cancelled during shutdown
	listenErrs := make(chan error, len(servers))
	for _, s := range servers {
		go func(s namedServer) {
			logger.Info(logCtx, "starting server", zap.String("server", s.name))
			if err := s.listen(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				listenErrs <- fmt.Errorf("%s server: %w", s.name, err)
			}
		}(s)
	}

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info(logCtx, "shutting down", zap.Duration("timeout", timeout))
	case runErr = <-listenErrs:
		logger.Error(logCtx, "server failed; shutting down", zap.Error(runErr))
	}

	begin := time.Now()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(s namedServer) {
			defer wg.Done()
			t := time.Now()
			if err := s.shutdown(shutdownCtx); err != nil {
				logger.Warn(logCtx, "server shutdown incomplete; open connections abandoned",
					zap.String("server", s.name), zap.Duration("took", time.Since(t)), zap.Error(err))
				return
			}
			logger.Info(logCtx, "server stopped",
				zap.String("server", s.name), zap.Duration("took", time.Since(t)))
		}(s)
	}
	wg.Wait()

	for _, s := range stops {
		t := time.Now()
		s.stop()
		logger.Info(logCtx, "worker stopped", zap.String("worker", s.name), zap.Duration("took", time.Since(t)))
	}
	logger.Info(logCtx, "shutdown complete", zap.Duration("took", time.Since(begin)))
	return runErr
}

// applyTrustedProxies sets the proxies whose X-Forwarded-For / X-Real-IP
// headers Gin believes. None configured means nil: the headers are ignored
// and ClientIP is the TCP peer, so clients cannot spoof click IPs.
func applyTrustedProxies(engine *gin.Engine, proxies []string) error {
	if len(proxies) == 0 {
		proxies = nil
	}
	return engine.SetTrustedProxies(proxies)
}
