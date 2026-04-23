package registry

import (
	"os"
	"os/signal"

	datacorekube "github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/rpc"
	rpchandler "github.com/mydecisive/octant/internal/rpc/handler"
	"go.uber.org/dig"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/kubernetes"
)

// Initialize adds all the dependencies to DI.
func Initialize() (*dig.Container, error) {
	container := dig.New(dig.DeferAcyclicVerification())
	if err := container.Provide(config.Read); err != nil {
		return nil, err
	}

	if err := container.Invoke(initLogger); err != nil {
		return nil, err
	}

	if err := container.Provide(provideKubeClient); err != nil {
		return nil, err
	}
	if err := container.Provide(integration.NewDataDogIntegration, dig.As(new(integration.Integration[integration.DataDogIntegrationData]))); err != nil {
		return nil, err
	}
	if err := container.Provide(integration.NewArgoCDIntegration, dig.As(new(integration.Integration[integration.ArgoCDIntegrationData]))); err != nil {
		return nil, err
	}
	if err := container.Provide(argocd.NewArgoCDClient, dig.As(new(argocd.APIClient))); err != nil {
		return nil, err
	}

	// RPC Server
	if err := container.Provide(rpchandler.NewArgoCDHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(rpchandler.NewInstallHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(rpchandler.NewDatadogHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(rpc.NewServer); err != nil {
		return nil, err
	}
	return container, nil
}

// SetupGracefulShutdown will create a thread that traps shutdown signal
// to allow the service to do any finishing work before shutdown.
func SetupGracefulShutdown() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGTSTP)
	go func() {
		<-sigs
		signal.Stop(sigs)
		close(sigs)

		// Stop whole system
		zap.L().Info("Shutting down...")
		zap.L().Sync() // nolint:errcheck
		os.Exit(0)     // nolint:forbidigo, revive //must call os.Exit
	}()
}

// initLogger setup global zap logger.
func initLogger(configuration *config.Configuration) error {
	var logger *zap.Logger
	var err error
	if configuration.Env == config.Prod {
		logger, err = zap.NewProduction(zap.AddStacktrace(zap.PanicLevel))
		if err != nil {
			return err
		}
	} else {
		logger, err = zap.NewDevelopment(zap.AddStacktrace(zap.PanicLevel))
		if err != nil {
			return err
		}
	}
	zap.ReplaceGlobals(logger)
	zap.RedirectStdLog(logger)
	return nil
}

func provideKubeClient() (kubernetes.Interface, error) {
	return datacorekube.NewK8sClient(zap.L())
}
