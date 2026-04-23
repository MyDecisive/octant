//go:build webapp
// +build webapp

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/mydecisive/mdai-data-core/helpers"
)

//go:embed dist/*
var App embed.FS

const (
	httpPortEnvVarKey = "HTTP_PORT"
	defaultHTTPPort   = "5678"

	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 10 * time.Second
	defaultIdleTimeout       = 120 * time.Second
)

func CreateServer() (*http.Server, error) {
	mainRouter := http.NewServeMux()

	octantApp, err := fs.Sub(App, "dist")
	if err != nil {
		return nil, err
	}

	// octant UI
	mainRouter.Handle("/", http.FileServerFS(octantApp))

	httpPort := helpers.GetEnvVariableWithDefault(httpPortEnvVarKey, defaultHTTPPort)

	return &http.Server{
		Addr:              ":" + httpPort,
		Handler:           mainRouter,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}, nil
}
