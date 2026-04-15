//go:build webapp
// +build webapp

package main

import (
	"io/fs"
	"net/http"
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
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	mainRouter := http.NewServeMux()

	octantApp, err := fs.Sub(web.App, "dist")
	if err != nil {
		logger.Fatal("failed to load embedded octant UI", zap.Error(err))
	}

	apiRouter := http.NewServeMux()
	apiRouter.HandleFunc("GET /health", func(writer http.ResponseWriter, request *http.Request) {
		if _, err = writer.Write([]byte("OK")); err != nil {
			logger.Error("failed to write health response", zap.Error(err))
		}
	})

	// octant UI
	mainRouter.Handle("/", http.FileServerFS(octantApp))
	// octant API
	mainRouter.Handle("/api/v1/", http.StripPrefix("/api/v1", apiRouter))

	httpPort := helpers.GetEnvVariableWithDefault(httpPortEnvVarKey, defaultHTTPPort)
	logger.Info("starting server", zap.String("address", ":"+httpPort))

	httpServer := &http.Server{
		Addr:              ":" + httpPort,
		Handler:           mainRouter,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	logger.Fatal("failed to start server", zap.Error(httpServer.ListenAndServe()))
}
