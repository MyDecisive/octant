package main

import (
	"github.com/mydecisive/octant/internal/config"
	"go.uber.org/zap"
	"log"
)

func setup() (*zap.Logger, *config.Configuration, func()) {
	configuration, err := config.Read()
	if err != nil {
		log.Fatalf("reading config: %v\n", err) // nolint:forbidigo // zap not setup yet
	}

	// Setup logger
	var logger *zap.Logger
	if configuration.Env == config.Prod {
		logger, err = zap.NewProduction(zap.AddStacktrace(zap.PanicLevel))
		if err != nil {
			log.Fatalf("Setup logger: %v\n", err) // nolint:forbidigo // zap not setup yet
		}
	} else {
		logger, err = zap.NewDevelopment(zap.AddStacktrace(zap.PanicLevel))
		if err != nil {
			log.Fatalf("Setup logger: %v\n", err) // nolint:forbidigo // zap not setup yet
		}
	}

	undo := zap.ReplaceGlobals(logger)
	reset := zap.RedirectStdLog(logger)
	return logger, configuration, func() {
		if err = logger.Sync(); err != nil {
			logger.Error("syncing logger", zap.Error(err))
		}
		undo()
		reset()
	}
}
