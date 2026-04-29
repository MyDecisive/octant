package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mydecisive/octant/internal/config"
)

func TestNewPromClientFactory(t *testing.T) {
	t.Parallel()
	configuration := config.Configuration{
		Metrics: config.Metrics{},
	}
	factory := NewPromClientFactory(&configuration)

	// Verify map is initialized (sync.Map is usable zero-value)
	if factory == nil {
		t.Fatal("expected factory to not be nil")
	}
}

func TestGetPromClient_ReturnsValidClient(t *testing.T) {
	t.Parallel()
	configuration := config.Configuration{
		Metrics: config.Metrics{
			PrometheusServiceName: "foo",
			PrometheusPort:        1234,
		},
	}
	factory := NewPromClientFactory(&configuration)
	namespace := "default"

	client, err := factory.GetPromClient(namespace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to not be nil")
	}
}

func TestGetPromClient_CachingWorks(t *testing.T) {
	t.Parallel()
	configuration := config.Configuration{
		Metrics: config.Metrics{
			PrometheusServiceName: "foo",
			PrometheusPort:        1234,
		},
	}
	factory := NewPromClientFactory(&configuration)
	namespace := "monitoring"

	// First call to populate the cache
	client1, err := factory.GetPromClient(namespace)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Second call should fetch from cache
	client2, err := factory.GetPromClient(namespace)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	// Because it returns an interface, we verify they point to the same underlying implementation instance
	if client1 != client2 {
		t.Error("expected cached client to be the exact same instance, but got different ones")
	}
}

func TestGetPromClient_InvalidCacheType(t *testing.T) {
	t.Parallel()
	configuration := config.Configuration{
		Metrics: config.Metrics{},
	}
	factory := NewPromClientFactory(&configuration)
	namespace := "invalid-cache-ns"

	// Manually inject a bad type into the cache
	factory.cache.Store(namespace, "this-is-a-string-not-an-api-client")

	client, err := factory.GetPromClient(namespace)
	if client != nil {
		t.Errorf("expected client to be nil, got %v", client)
	}
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if err.Error() != "cannot use cached client" {
		t.Errorf("expected 'cannot use cached client' error, got: %v", err)
	}
}

func TestGetPromClient_UsesEnvVar(t *testing.T) {
	t.Parallel()
	// Set up a mock HTTP server to intercept the API call
	serverHit := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return a valid empty Prometheus JSON response so the client parser doesn't error
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`)) // nolint: errcheck
	}))
	defer mockServer.Close()

	configuration := config.Configuration{
		Metrics: config.Metrics{
			PrometheusURLOverride: mockServer.URL,
		},
	}
	factory := NewPromClientFactory(&configuration)
	namespace := "test-env"

	client, err := factory.GetPromClient(namespace)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to not be nil")
	}

	// Execute a lightweight query to force the client to make a network request
	_, _, err = client.Query(context.Background(), "up", time.Now())
	if err != nil {
		t.Fatalf("unexpected error during query: %v", err)
	}

	// Verify the mock server received the request
	if !serverHit {
		t.Error("The DEV_PROMETHEUS_URL environment variable was not used; mock server was never hit")
	}
}

func TestGetPromClient_ApiNewClientError(t *testing.T) {
	t.Parallel()
	// Set a fundamentally invalid URL scheme to trigger api.NewClient error
	// The prometheus client uses url.Parse under the hood
	configuration := config.Configuration{
		Metrics: config.Metrics{
			PrometheusURLOverride: "://bad-url",
		},
	}
	factory := NewPromClientFactory(&configuration)
	namespace := "bad-url-ns"

	client, err := factory.GetPromClient(namespace)
	if client != nil {
		t.Errorf("expected client to be nil on error, got %v", client)
	}
	if err == nil {
		t.Fatal("expected an error due to invalid URL, got nil")
	}

	expectedErrPrefix := "failed to create prometheus client for namespace bad-url-ns"
	if len(err.Error()) < len(expectedErrPrefix) || err.Error()[:len(expectedErrPrefix)] != expectedErrPrefix {
		t.Errorf("expected error to start with '%s', got: %v", expectedErrPrefix, err)
	}
}
