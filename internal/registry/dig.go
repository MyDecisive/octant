package registry

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	datacorekube "github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/budget"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
	budgetdb "github.com/mydecisive/octant/internal/budget/data/db"
	budgetfilter "github.com/mydecisive/octant/internal/budget/filter"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/metrics"
	"github.com/mydecisive/octant/internal/rpc"
	rpchandler "github.com/mydecisive/octant/internal/rpc/handler"
	"github.com/mydecisive/octant/internal/setting"
	"github.com/mydecisive/octant/internal/wrapper"
	"go.uber.org/dig"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/kubernetes"
)

// Initialize adds all the dependencies to DI.
func Initialize() (*dig.Container, error) { // nolint: cyclop,funlen // yes, we will have a lot if statements in here...
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
	if err := container.Provide(provideHTTPClient); err != nil {
		return nil, err
	}
	if err := container.Provide(metrics.NewPromClientFactory, dig.As(new(metrics.PromClientFactory))); err != nil {
		return nil, err
	}
	if err := container.Provide(metrics.NewPrometheusConnectionStatus, dig.As(new(metrics.ConnectionStatus))); err != nil {
		return nil, err
	}
	if err := container.Provide(provideConfigMapController); err != nil {
		return nil, err
	}
	if err := container.Provide(provideSecretController); err != nil {
		return nil, err
	}

	// Budget
	if err := container.Provide(
		budgetdata.NewMDAIGateway,
		dig.As(new(budgetdata.VariableAccessor))); err != nil {
		return nil, err
	}
	if err := container.Provide(
		budgetfilter.NewMDAISettingController,
		dig.As(new(budgetfilter.SettingController))); err != nil {
		return nil, err
	}
	if err := container.Provide(
		budgetdb.NewGreptimeDBBuilder,
		dig.As(new(budgetdb.DatabaseAccessBuilder))); err != nil {
		return nil, err
	}
	if err := container.Provide(
		budgetdata.NewGreptimeDataRetriever,
		dig.As(new(budgetdata.MetricDataRetriever))); err != nil {
		return nil, err
	}
	if err := container.Provide(
		budget.NewMetricProvider,
		dig.As(new(budget.MetricDataProvider))); err != nil {
		return nil, err
	}

	// Integration
	if err := container.Provide(
		integration.NewDataDogIntegration,
		dig.As(new(integration.Integration[integration.DataDogIntegrationData]))); err != nil {
		return nil, err
	}
	if err := container.Provide(
		integration.NewArgoCDIntegration,
		dig.As(new(integration.Integration[integration.ArgoCDIntegrationData]))); err != nil {
		return nil, err
	}
	if err := container.Provide(
		argocd.NewArgoCDClient,
		dig.As(new(argocd.APIClient))); err != nil {
		return nil, err
	}
	if err := container.Provide(
		connection.NewConnectionManifestGenerator,
		dig.As(new(connection.ManifestGenerator))); err != nil {
		return nil, err
	}
	if err := container.Provide(
		connection.NewConnectionManifestCompressor,
		dig.As(new(connection.ManifestCompressor))); err != nil {
		return nil, err
	}

	// Connection
	if err := container.Provide(provideOctantConnection); err != nil {
		return nil, err
	}
	if err := container.Provide(setting.NewSettingManagerBuilder, dig.As(new(setting.ManagerBuilder))); err != nil {
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
	if err := container.Provide(rpchandler.NewConnectionHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(rpchandler.NewBudgetFilterHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(rpchandler.NewBudgetTimeframeHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(rpchandler.NewBudgetHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(rpchandler.NewSettingHandler); err != nil {
		return nil, err
	}
	if err := container.Provide(rpc.NewServer); err != nil {
		return nil, err
	}
	return container, nil
}

// SetupGracefulShutdown will create a thread that traps shutdown signal
// to allow the service to do any finishing work before shutdown.
func SetupGracefulShutdown(container *dig.Container) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGTSTP)
	go func() {
		<-sigs
		signal.Stop(sigs)
		close(sigs)

		if err := container.Invoke(func(cmStore datacorekube.ConfigMapStore, secretStore datacorekube.SecretStore) {
			cmStore.Stop()
			secretStore.Stop()
		}); err != nil {
			zap.L().Error("unexpected error stopping k8s informers", zap.Error(err))
		}

		// Stop whole system
		zap.L().Info("Shutting down...")
		zap.L().Sync() // nolint:errcheck
		os.Exit(0)     // nolint:forbidigo, revive //must call os.Exit
	}()
}

// initLogger setup global zap logger.
func initLogger(configuration *config.Configuration) error {
	newLogger := zap.NewDevelopment
	if configuration.Env == config.Prod {
		newLogger = zap.NewProduction
	}

	logger, err := newLogger(zap.AddStacktrace(zap.PanicLevel))
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)
	zap.RedirectStdLog(logger)
	return nil
}

func provideKubeClient() (kubernetes.Interface, error) { // nolint: ireturn
	return datacorekube.NewK8sClient(zap.L())
}

func provideHTTPClient(configuration *config.Configuration) wrapper.HTTPClient { // nolint: ireturn
	return &http.Client{
		Timeout: time.Duration(configuration.DefaultTimeout) * time.Second,
	}
}

func provideConfigMapController( // nolint: ireturn
	theConfig *config.Configuration,
	k8sClient kubernetes.Interface,
) (datacorekube.ConfigMapStore, error) {
	controller, err := datacorekube.NewConfigMapController(
		[]string{datacorekube.OctantConnectionsConfigMapType},
		theConfig.CurrentNamespace,
		k8sClient,
		zap.L(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ConfigMap controller: %w", err)
	}

	if err = controller.Run(); err != nil {
		return nil, fmt.Errorf("failed to sync configmap controller cache: %w", err)
	}
	return controller, nil
}

func provideSecretController( // nolint: ireturn
	theConfig *config.Configuration,
	k8sClient kubernetes.Interface,
) (datacorekube.SecretStore, error) {
	controller, err := datacorekube.NewSecretController([]string{
		datacorekube.OctantIntegrationArgoType,
		datacorekube.OctantIntegrationDatadogType,
	}, theConfig.CurrentNamespace, k8sClient, zap.L())
	if err != nil {
		return nil, fmt.Errorf("failed to create Secret controller: %w", err)
	}

	if err = controller.Run(); err != nil {
		return nil, fmt.Errorf("failed to sync Secret controller cache: %w", err)
	}
	return controller, nil
}

func provideOctantConnection(
	cmStore datacorekube.ConfigMapStore,
	theConfig *config.Configuration,
	k8sClient kubernetes.Interface,
	connectionMetrics metrics.ConnectionStatus,
	manifestGenerator connection.ManifestGenerator,
	argocdIntegration integration.Integration[integration.ArgoCDIntegrationData],
	datadogIntegration integration.Integration[integration.DataDogIntegrationData],
	argoClient argocd.APIClient,
) connection.Connection[connection.OctantConnectionData] { // nolint: ireturn
	return connection.NewOctantConnection(
		cmStore,
		theConfig,
		connection.WithK8sClient(k8sClient),
		connection.WithConnectionMetrics(connectionMetrics),
		connection.WithGenerator(manifestGenerator),
		connection.WithArgoCDIntegration(argocdIntegration),
		connection.WithArgoClient(argoClient),
		connection.WithDatadogIntegration(datadogIntegration),
	)
}
