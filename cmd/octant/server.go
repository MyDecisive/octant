package main

import (
	datacorekube "github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/config"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"log"
	"os"
	"strings"
)

const namespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

type dependencies struct {
	logger       *zap.Logger
	config       *config.Configuration
	k8sClient    kubernetes.Interface
	k8sNamespace string
}

func setup() (dependencies, func()) {
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

	clientset, err := datacorekube.NewK8sClient(logger)
	if err != nil {
		logger.Fatal("creating kubernetes client: %w", zap.Error(err))
	}
	return dependencies{
			logger:       logger,
			config:       configuration,
			k8sClient:    clientset,
			k8sNamespace: getCurrentNamespace(),
		}, func() {
			if err = logger.Sync(); err != nil {
				logger.Error("syncing logger", zap.Error(err))
			}
			undo()
			reset()
		}
}

func getCurrentNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	if data, err := os.ReadFile(namespaceFilePath); err == nil {
		ns := strings.TrimSpace(string(data))
		if ns != "" {
			return ns
		}
	}

	return "default"
}
