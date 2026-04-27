package metrics

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mydecisive/octant/internal/config"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1" // nolint: goimports
)

type PromClientFactory interface {
	GetPromClient(namespace string) (v1.API, error)
}

type PromClientFactoryImpl struct {
	cache         sync.Map
	configuration *config.Configuration
}

func NewPromClientFactory(configuration *config.Configuration) *PromClientFactoryImpl {
	return &PromClientFactoryImpl{
		configuration: configuration,
	}
}

func (f *PromClientFactoryImpl) GetPromClient(namespace string) (v1.API, error) { // nolint: ireturn
	if cachedClient, ok := f.cache.Load(namespace); ok {
		client, ok := cachedClient.(v1.API)
		if !ok {
			return nil, errors.New("cannot use cached client")
		}
		return client, nil
	}

	promURL := f.configuration.Metrics.PrometheusURLOverride
	if promURL == "" {
		if f.configuration.Metrics.PrometheusServiceName == "" || f.configuration.Metrics.PrometheusPort == 0 {
			return nil, errors.New("prometheus service name and prometheus service name must be defined")
		}
		promURL = fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d", //nolint:revive
			f.configuration.Metrics.PrometheusServiceName,
			namespace,
			f.configuration.Metrics.PrometheusPort,
		)
	}

	client, err := api.NewClient(api.Config{
		Address: promURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client for namespace %s: %w", namespace, err)
	}

	promAPI := v1.NewAPI(client)

	actualClient, _ := f.cache.LoadOrStore(namespace, promAPI)
	newClient, ok := actualClient.(v1.API)
	if !ok {
		return nil, errors.New("cannot use cached client")
	}
	return newClient, nil
}
