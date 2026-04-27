package connection

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mydecisive/octant/internal/integration"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	metricsmock "github.com/mydecisive/octant/internal/mock/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const defaultNamespace = "default"

// --- MOCKS ---

// Helper to stand up a fake Argo API.
func setupTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Return success for all standard operations
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/applications/team-a":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": {"health": {"status": "Healthy"}}}`)) // nolint: errcheck,gosec,revive
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/applications":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/applications/team-a/sync":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/applications/team-a":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK) // Catch-all for tests not strictly checking response bodies
		}
	}))
}

// --- FIXTURE HELPER ---

type octantTestFixture struct {
	k8sClient       *fake.Clientset
	argoMock        *integrationmock.MockIntegration[integration.ArgoCDIntegrationData]
	datadogMock     *integrationmock.MockIntegration[integration.DataDogIntegrationData]
	promFactoryMock *metricsmock.MockPromClientFactory
	httpClient      *http.Client
}

// setupFixture initializes a default set of dependencies for OctantConnection.
// It accepts optional runtime objects to seed the fake Kubernetes client.
func setupFixture(t *testing.T, objects ...runtime.Object) *octantTestFixture {
	t.Helper()
	return &octantTestFixture{
		k8sClient:       fake.NewClientset(objects...),
		argoMock:        integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t),
		datadogMock:     integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t),
		promFactoryMock: metricsmock.NewMockPromClientFactory(t),
		httpClient:      http.DefaultClient, // Default safe client; tests can override with httptest server clients
	}
}

// build creates the OctantConnection with the current state of the fixture.
func (f *octantTestFixture) build() *OctantConnection {
	return NewOctantConnection(
		f.httpClient,
		f.k8sClient,
		f.argoMock,
		f.datadogMock,
		f.promFactoryMock,
	)
}

// --- TESTS ---

func TestGetConnectionByName(t *testing.T) {
	t.Parallel()

	ts := setupTestServer()
	defer ts.Close()

	validConnection := OctantConnectionData{
		SourceType: "datadog",
		TelemetryTypes: []telemetry.MLT{
			telemetry.Logs,
			telemetry.Traces,
		},
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
	}
	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": string(validConnectionBytes),
		},
	})
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, validConnection.Deployment.IntegrationName).
		Return(&integration.ArgoCDIntegrationData{
			APIUrl:       ts.URL,
			AccountToken: "fake-token",
		}, nil)

	octantConnection := f.build()

	actual, getErr := octantConnection.GetConnectionByName(context.Background(), defaultNamespace, "team-a")
	require.NoError(t, getErr)
	require.NotNil(t, actual)

	statusMap, ok := actual.Status.(*argoApp)
	require.True(t, ok)
	assert.Equal(t, "Healthy", statusMap.Status.Health.Status)
}

func TestGetConnectionByName_NotFound_NoConfigMap(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	octantConnection := f.build()

	actual, err := octantConnection.GetConnectionByName(context.Background(), defaultNamespace, "team-a")

	require.NoError(t, err)
	assert.Nil(t, actual)
}

func TestGetConnectionByName_NotFound_KeyMissing(t *testing.T) {
	t.Parallel()

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-b": `{"sourceType": "datadog"}`, // Populating team-b
		},
	})

	octantConnection := f.build()

	actual, err := octantConnection.GetConnectionByName(context.Background(), defaultNamespace, "team-a")

	require.NoError(t, err)
	assert.Nil(t, actual)
}

func TestGetConnectionByName_Error_ConfigMapGetFailed(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	f.k8sClient.PrependReactor("get", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("injected get error")
	})

	octantConnection := f.build()

	_, err := octantConnection.GetConnectionByName(context.Background(), defaultNamespace, "team-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get configmap")
	assert.Contains(t, err.Error(), "injected get error")
}

func TestGetConnectionByName_Error_InvalidJSON(t *testing.T) {
	t.Parallel()

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": "not gonna work",
		},
	})

	octantConnection := f.build()

	_, err := octantConnection.GetConnectionByName(context.Background(), defaultNamespace, "team-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal connection data")
}

func TestGetConnectionByName_Error_ArgoStatusFailed(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	validConnection := OctantConnectionData{
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
	}
	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": string(validConnectionBytes),
		},
	})
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, validConnection.Deployment.IntegrationName).
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)

	octantConnection := f.build()

	_, err = octantConnection.GetConnectionByName(context.Background(), defaultNamespace, "team-a")
	require.Error(t, err)
}

func TestSaveConnection(t *testing.T) {
	t.Parallel()

	ts := setupTestServer()
	defer ts.Close()

	newConnection := OctantConnectionData{
		SourceType: "datadog",
		Destinations: []OctantConnectionDestination{
			{DestinationType: "datadog", IntegrationName: "dd-test"},
		},
		TelemetryTypes: []telemetry.MLT{
			telemetry.Logs,
			telemetry.Traces,
		},
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
	}

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, newConnection.Deployment.IntegrationName).
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)
	f.datadogMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, newConnection.Destinations[0].IntegrationName).
		Return(&integration.DataDogIntegrationData{}, nil)

	octantConnection := f.build()

	err := octantConnection.SaveConnection(context.Background(), newConnection, defaultNamespace, "team-a")
	require.NoError(t, err)

	cm, err := f.k8sClient.CoreV1().
		ConfigMaps(defaultNamespace).
		Get(context.Background(), connectionsConfigmapName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Contains(t, cm.Data, "team-a")
}

func TestSaveConnection_UpdateExistingConfigMap(t *testing.T) {
	t.Parallel()

	ts := setupTestServer()
	defer ts.Close()

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"existing-team": `{"sourceType": "datadog", 
			"telemetryTypes": ["logs", "traces"], 
			"deployment": {"type": "argocd", "fields": {"branch": "tv/coolBranch"}}}`,
		},
	})
	f.httpClient = ts.Client()

	newConnection := OctantConnectionData{
		Deployment: &Deployment{
			Type: ArgoManifestsDeploymentType,
		},
	}

	octantConnection := f.build()

	err := octantConnection.SaveConnection(context.Background(), newConnection, defaultNamespace, "team-a")
	require.NoError(t, err)

	cm, err := f.k8sClient.CoreV1().
		ConfigMaps(defaultNamespace).
		Get(context.Background(), connectionsConfigmapName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Contains(t, cm.Data, "team-a")
	require.Contains(t, cm.Data, "existing-team")
}

func TestSaveConnection_Error_InvalidDeploymentType(t *testing.T) {
	t.Parallel()

	invalidConnection := OctantConnectionData{
		Deployment: &Deployment{
			Type: "invalid-deployment-type",
		},
	}

	f := setupFixture(t)
	octantConnection := f.build()

	err := octantConnection.SaveConnection(context.Background(), invalidConnection, defaultNamespace, "team-a")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid deployment type: invalid-deployment-type")
}

func TestSaveConnection_Error_ConfigMapGetFailed(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	f.k8sClient.PrependReactor("get", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("injected get error")
	})

	octantConnection := f.build()

	validConnection := OctantConnectionData{
		Deployment: &Deployment{
			Type: ArgoManifestsDeploymentType,
		},
	}

	err := octantConnection.SaveConnection(context.Background(), validConnection, defaultNamespace, "team-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch configmap")
}

func TestSaveConnection_Error_ArgoPushFailed(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	connection := OctantConnectionData{
		Destinations: []OctantConnectionDestination{
			{DestinationType: "datadog", IntegrationName: "dd-1"},
		},
		Deployment: &Deployment{
			Type: ArgoSideloadDeploymentType,
		},
	}

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data:       map[string]string{},
	})
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, connection.Deployment.IntegrationName).
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)
	f.datadogMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, connection.Destinations[0].IntegrationName).
		Return(&integration.DataDogIntegrationData{}, nil)

	octantConnection := f.build()

	err := octantConnection.SaveConnection(context.Background(), connection, defaultNamespace, "team-a")
	require.Error(t, err)
}

func TestDeleteConnection(t *testing.T) {
	t.Parallel()

	ts := setupTestServer()
	defer ts.Close()

	existingConnection := OctantConnectionData{
		SourceType: "datadog",
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
	}
	existingConnectionBytes, marshalErr := json.Marshal(existingConnection)
	require.NoError(t, marshalErr)

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": string(existingConnectionBytes),
		},
	})
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, existingConnection.Deployment.IntegrationName).
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)

	octantConnection := f.build()

	deleteErr := octantConnection.DeleteConnection(context.Background(), defaultNamespace, "team-a")
	require.NoError(t, deleteErr)

	cm, getCMErr := f.k8sClient.CoreV1().
		ConfigMaps(defaultNamespace).
		Get(context.Background(), connectionsConfigmapName, metav1.GetOptions{})
	require.NoError(t, getCMErr)
	require.NotContains(t, cm.Data, "team-a")
}

func TestDeleteConnection_NotFound_SilentlyReturns(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	octantConnection := f.build()

	err := octantConnection.DeleteConnection(context.Background(), defaultNamespace, "team-a")
	require.NoError(t, err)
}

func TestDeleteConnection_KeyMissing_SilentlyReturns(t *testing.T) {
	t.Parallel()

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-b": `{"sourceType": "datadog"}`,
		},
	})
	octantConnection := f.build()

	err := octantConnection.DeleteConnection(context.Background(), defaultNamespace, "team-a")
	require.NoError(t, err)
}

func TestDeleteConnection_Error_ConfigMapGetFailed(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	f.k8sClient.PrependReactor("get", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("injected get error")
	})

	octantConnection := f.build()

	err := octantConnection.DeleteConnection(context.Background(), defaultNamespace, "team-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch configmap")
}

func TestDeleteConnection_Error_InvalidJSON(t *testing.T) {
	t.Parallel()

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": "{ invalid json ",
		},
	})
	octantConnection := f.build()

	err := octantConnection.DeleteConnection(context.Background(), defaultNamespace, "team-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal connection data")
}

func TestDeleteConnection_Error_ArgoDeleteFailed(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	connection := OctantConnectionData{
		Deployment: &Deployment{
			Type: ArgoSideloadDeploymentType,
		},
	}
	connBytes, err := json.Marshal(connection)
	require.NoError(t, err)

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": string(connBytes),
		},
	})
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(
			mock.Anything,
			defaultNamespace,
			connection.Deployment.IntegrationName).
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)

	octantConnection := f.build()

	deleteErr := octantConnection.DeleteConnection(context.Background(), defaultNamespace, "team-a")
	require.Error(t, deleteErr)
}

func TestDeleteConnection_Error_ConfigMapUpdateFailed(t *testing.T) {
	t.Parallel()

	connection := OctantConnectionData{}
	connBytes, err := json.Marshal(connection)
	require.NoError(t, err)

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": string(connBytes),
		},
	})
	f.k8sClient.PrependReactor("update", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("injected update error")
	})

	octantConnection := f.build()

	updateErr := octantConnection.DeleteConnection(context.Background(), defaultNamespace, "team-a")
	require.Error(t, updateErr)
	assert.Contains(t, updateErr.Error(), "failed to update configmap")
}

func TestGetConnectionStatus_Success(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType: "datadog",
		TelemetryTypes: []telemetry.MLT{
			telemetry.Logs,
		},
	}
	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": string(validConnectionBytes),
		},
	})

	promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseString := `{"status":"success",
		"data":{"resultType":"vector","result":[{"metric":{"signal":"logs","result":"pass"},"value":[1712419691,"5"]}]}}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseString)) //nolint: errcheck,gosec,revive
	}))
	defer promServer.Close()

	promClient, err := api.NewClient(api.Config{Address: promServer.URL})
	require.NoError(t, err)
	promAPI := promv1.NewAPI(promClient)

	// Set up the PromClientFactory mock to return our test PromAPI
	f.promFactoryMock.EXPECT().GetPromClient(defaultNamespace).Return(promAPI, nil).Times(1)

	octantConnection := f.build()

	status, err := octantConnection.GetConnectionStatus(context.Background(), defaultNamespace, "team-a")

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.ReceivingData)
	assert.True(t, status.SendingData)
	assert.True(t, status.DataIntegrity)
}

func TestGetConnectionStatus_Error_PrometheusFailed(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		TelemetryTypes: []telemetry.MLT{telemetry.Logs},
	}
	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	f := setupFixture(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
		Data: map[string]string{
			"team-a": string(validConnectionBytes),
		},
	})

	// Mock Prometheus server returning a 500 error
	promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer promServer.Close()

	promClient, err := api.NewClient(api.Config{Address: promServer.URL})
	require.NoError(t, err)
	promAPI := promv1.NewAPI(promClient)

	// Set up the PromClientFactory mock to return our test PromAPI
	f.promFactoryMock.EXPECT().GetPromClient(defaultNamespace).Return(promAPI, nil).Times(1)

	octantConnection := f.build()

	status, err := octantConnection.GetConnectionStatus(context.Background(), defaultNamespace, "team-a")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "querying telemetry")
	assert.Nil(t, status)
}

func TestGetConnectionStatus_Error_K8sGetFailed(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	octantConnection := f.build()

	// Force a hard K8s error so GetConnectionByName returns an actual error
	f.k8sClient.PrependReactor("get", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("k8s api failure")
	})

	status, err := octantConnection.GetConnectionStatus(context.Background(), defaultNamespace, "team-a")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting connection")
	assert.Contains(t, err.Error(), "k8s api failure")
	assert.Nil(t, status)
}

func TestGetConnectionStatus_NotFound_ReturnsError(t *testing.T) {
	t.Parallel()

	f := setupFixture(t) // Empty fixture, ConfigMap doesn't exist
	octantConnection := f.build()

	status, err := octantConnection.GetConnectionStatus(context.Background(), defaultNamespace, "missing-team")

	require.Error(t, err)
	assert.Nil(t, status)
}
