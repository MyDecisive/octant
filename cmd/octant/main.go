//go:build !webapp

package main

import (
	"net/http"
	"time"

	"github.com/mydecisive/mdai-data-core/helpers"
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

	mainRouter := setupRouter(logger)

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
