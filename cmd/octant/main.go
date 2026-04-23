package main

import (
	"log"

	"github.com/mydecisive/octant/internal/registry"
	"go.uber.org/zap"
)

func main() {
	container, err := registry.Initialize()
	if err != nil {
		log.Fatalf("Failed to setup: %v\n", err) //nolint:forbidigo //no zap yet
	}
	registry.SetupGracefulShutdown()

	if err := container.Invoke(registry.Start); err != nil {
		zap.L().Fatal("Start servers", zap.Error(err))
	}
}
