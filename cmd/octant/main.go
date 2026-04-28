package main

import (
	"github.com/mydecisive/octant/internal/registry"
	"go.uber.org/zap"
)

func main() {
	container, err := registry.Initialize()
	if err != nil {
		zap.L().Fatal("Failed to setup", zap.Error(err))
	}
	registry.SetupGracefulShutdown()

	if err = container.Invoke(registry.Start); err != nil {
		zap.L().Fatal("Start servers", zap.Error(err))
	}
}
