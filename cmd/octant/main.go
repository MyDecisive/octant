package main

import (
	"github.com/mydecisive/octant/web"
	"go.uber.org/zap"
	"io/fs"
	"net/http"

	"github.com/mydecisive/mdai-data-core/helpers"
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
		_, err = writer.Write([]byte("OK"))
		if err != nil {
			logger.Error("failed to write health response", zap.Error(err))
		}
	})

	// octant UI
	mainRouter.Handle("/", http.FileServerFS(octantApp))
	// octant API
	mainRouter.Handle("/api/v1/", http.StripPrefix("/api/v1", apiRouter))

	httpPort := helpers.GetEnvVariableWithDefault("HTTP_PORT", "5678")
	logger.Info("starting server", zap.String("address", ":"+httpPort))

	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: mainRouter,
	}

	logger.Fatal("failed to start server", zap.Error(httpServer.ListenAndServe()))
}
