package main

import (
	"net/http"

	"go.uber.org/zap"
)

func setupRouter(logger *zap.Logger) *http.ServeMux {
	mainRouter := http.NewServeMux()

	apiRouter := http.NewServeMux()
	apiRouter.HandleFunc("GET /health", func(writer http.ResponseWriter, request *http.Request) {
		_, err := writer.Write([]byte("OK"))
		if err != nil {
			logger.Error("failed to write health response", zap.Error(err))
		}
	})

	// octant API
	mainRouter.Handle("/api/v1/", http.StripPrefix("/api/v1", apiRouter))
	return mainRouter
}
