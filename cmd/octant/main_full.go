//go:build webapp
// +build webapp

package main

import (
	"context"
	"fmt"
	"github.com/mydecisive/octant/internal/rpc"
	rpchandler "github.com/mydecisive/octant/internal/rpc/handler"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/mydecisive/mdai-data-core/helpers"
	"github.com/mydecisive/octant/web"

	"go.uber.org/zap"
)

const (
	httpPortEnvVarKey = "HTTP_PORT"
	defaultHTTPPort   = "5678"

	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 10 * time.Second
	defaultIdleTimeout       = 120 * time.Second
)

func main() {
	logger, configuration, cleanup := setup()
	defer cleanup()

	mainRouter := http.NewServeMux()

	octantApp, err := fs.Sub(web.App, "dist")
	if err != nil {
		logger.Fatal("failed to load embedded octant UI", zap.Error(err))
	}

	// octant UI
	mainRouter.Handle("/", http.FileServerFS(octantApp))

	httpPort := helpers.GetEnvVariableWithDefault(httpPortEnvVarKey, defaultHTTPPort)

	httpServer := &http.Server{
		Addr:              ":" + httpPort,
		Handler:           mainRouter,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	// Init Servers
	g, _ := errgroup.WithContext(context.Background())
	rpcServer := rpc.NewServer(*configuration, rpchandler.NewArgoCDHandler(configuration), rpchandler.NewInstallHandler())

	// Setup graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGTSTP)
	go func() {
		<-sigs
		signal.Stop(sigs)
		close(sigs)

		// Stop whole system
		logger.Info("shutting down servers...")
		os.Exit(0) // nolint: forbidigo
	}()

	// Start servers
	g.Go(func() error {
		logger.Info("starting RPC server", zap.Int("port", int(configuration.RPC.Port)))
		return fmt.Errorf("rpc server: %w", rpcServer.Start())
	})
	g.Go(func() error {
		logger.Info("starting UI server", zap.String("port", httpPort))
		return fmt.Errorf("UI server: %w", httpServer.ListenAndServe())
	})

	if err = g.Wait(); err != nil {
		logger.Fatal("starting servers", zap.Error(err))
	}
}
