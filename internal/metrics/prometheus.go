package metrics

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

const (
	defaultPromPort = 9090
	defaultPromHost = "prometheus-operated"
)

type PromClientFactory interface {
	GetPromClient(namespace string) (v1.API, error)
}

type promClientFactoryImpl struct {
	cache       sync.Map
	serviceName string
	port        int
}

func NewPromClientFactory() PromClientFactory {
	return &promClientFactoryImpl{
		// TODO: Update this to be configurable
		serviceName: defaultPromHost,
		port:        defaultPromPort,
	}
}

func (f *promClientFactoryImpl) GetPromClient(namespace string) (v1.API, error) {
	if cachedClient, ok := f.cache.Load(namespace); ok {
		client, ok := cachedClient.(v1.API)
		if !ok {
			return nil, errors.New("cannot use cached client")
		}
		return client, nil
	}

	promURL := os.Getenv("DEV_PROMETHEUS_URL")
	if promURL == "" {
		promURL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", f.serviceName, namespace, f.port)
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
