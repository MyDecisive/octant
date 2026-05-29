package connection

import (
	"encoding/json"
	"testing"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/mdai-data-core/kube"
	kubemock "github.com/mydecisive/mdai-data-core/mock/kube"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	metricsmock "github.com/mydecisive/octant/internal/mock/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	telemetryTypes = []telemetry.MLT{
		telemetry.Logs,
		telemetry.Traces,
	}
	defaultNamespace    = "default"
	argoIntegrationData = &integration.ArgoCDIntegrationData{
		APIUrl:       "http://argo.com",
		AccountToken: "abc123",
	}
	ddIntegrationData = &integration.DataDogIntegrationData{
		APIKey: "abc123",
		DDUrl:  "http://dd.com",
	}
	testConfig = &config.Configuration{
		CurrentNamespace:   defaultNamespace,
		ServiceAccountName: "coolServiceAccount",
		Env:                config.Dev,
	}
)

func TestGetConnectionByName(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType:     "datadog",
		TelemetryTypes: telemetryTypes,
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
		MdaiNamespace: defaultNamespace,
		Destinations: []OctantConnectionDestination{
			{
				DestinationType: "datadog",
				IntegrationName: "argo-test",
			},
		},
	}

	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	theConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      connectionsConfigmapName,
			Namespace: defaultNamespace,
			Labels: map[string]string{
				kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
			},
		},
		Data: map[string]string{
			"argo-test": string(validConnectionBytes),
		},
	}

	t.Run("configmap not found", func(t *testing.T) {
		t.Parallel()

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, connectionsConfigmapName)
		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(nil, notFoundError).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		connectionData, getErr := octantConnection.GetConnectionByName(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
		require.Nil(t, connectionData)
	})

	t.Run("connection not found in configmap", func(t *testing.T) {
		t.Parallel()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		connectionData, getErr := octantConnection.GetConnectionByName(t.Context(), ConnectionCRUDInput{
			ConnectionName: "team-b",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "connection 'team-b' not found")
		require.Nil(t, connectionData)
	})

	t.Run("invalid connection data", func(t *testing.T) {
		t.Parallel()

		badConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      connectionsConfigmapName,
				Namespace: defaultNamespace,
				Labels: map[string]string{
					kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
				},
			},
			Data: map[string]string{
				"argo-test": "}",
			},
		}
		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(badConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		connectionData, getErr := octantConnection.GetConnectionByName(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "failed to unmarshal connection data")
		require.Nil(t, connectionData)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		connectionData, getErr := octantConnection.GetConnectionByName(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
		require.NotNil(t, connectionData)
	})
}

func TestSaveConnection(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType:     "datadog",
		TelemetryTypes: telemetryTypes,
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
		MdaiNamespace: defaultNamespace,
		Destinations: []OctantConnectionDestination{
			{
				DestinationType: "datadog",
				IntegrationName: "argo-test",
			},
		},
	}
	t.Run("happy path - updated existing connection", func(t *testing.T) {
		t.Parallel()

		validConnectionBytes, err := json.Marshal(validConnection)
		require.NoError(t, err)

		theConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      connectionsConfigmapName,
				Namespace: defaultNamespace,
				Labels: map[string]string{
					kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
				},
			},
			Data: map[string]string{
				"argo-test": string(validConnectionBytes),
			},
		}

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "argo-test").
			Return(argoIntegrationData, nil).
			Once()

		mockDatadogIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadogIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "argo-test").
			Return(ddIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Once()
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "argo-test", mock.Anything, false).
			Return(nil).
			Once()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		generator := NewConnectionManifestGenerator(testConfig)

		octantConnection := NewOctantConnection(
			mockCmStore,
			testConfig,
			WithK8sClient(fake.NewClientset(theConfigmap)),
			WithArgoCDIntegration(mockArgoIntegration),
			WithDatadogIntegration(mockDatadogIntegration),
			WithArgoClient(mockArgoClient),
			WithGenerator(generator),
		)
		require.NoError(t, octantConnection.SaveConnection(t.Context(), validConnection, ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}))
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "argo-test").
			Return(argoIntegrationData, nil).
			Once()

		mockDatadogIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadogIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "argo-test").
			Return(ddIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Once()
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "argo-test", mock.Anything, false).
			Return(nil).
			Once()

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, connectionsConfigmapName)
		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(nil, notFoundError).
			Once()

		generator := NewConnectionManifestGenerator(testConfig)

		octantConnection := NewOctantConnection(
			mockCmStore,
			testConfig,
			WithK8sClient(fake.NewClientset()),
			WithArgoCDIntegration(mockArgoIntegration),
			WithDatadogIntegration(mockDatadogIntegration),
			WithArgoClient(mockArgoClient),
			WithGenerator(generator),
		)
		require.NoError(t, octantConnection.SaveConnection(t.Context(), validConnection, ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}))
	})
}

func TestDeleteConnection(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType:     "datadog",
		TelemetryTypes: telemetryTypes,
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
		MdaiNamespace: defaultNamespace,
		Destinations: []OctantConnectionDestination{
			{
				DestinationType: "datadog",
				IntegrationName: "argo-test",
			},
		},
	}

	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	theConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      connectionsConfigmapName,
			Namespace: defaultNamespace,
			Labels: map[string]string{
				kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
			},
		},
		Data: map[string]string{
			"argo-test": string(validConnectionBytes),
		},
	}

	t.Run("configmap not found", func(t *testing.T) {
		t.Parallel()

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, connectionsConfigmapName)
		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(nil, notFoundError).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		require.ErrorContains(t, octantConnection.DeleteConnection(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test-invalid",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}), "failed to fetch configmap")
	})

	t.Run("connection name doesn't exist", func(t *testing.T) {
		t.Parallel()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		require.ErrorContains(t, octantConnection.DeleteConnection(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test-invalid",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}), "connection 'argo-test-invalid' not found")
	})

	t.Run("error unmarshalling connection data", func(t *testing.T) {
		t.Parallel()

		badConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      connectionsConfigmapName,
				Namespace: defaultNamespace,
				Labels: map[string]string{
					kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
				},
			},
			Data: map[string]string{
				"argo-test": "}",
			},
		}

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(badConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		require.ErrorContains(t, octantConnection.DeleteConnection(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}), "failed to unmarshal connection data")
	})

	t.Run("happy path - not sideload deployment", func(t *testing.T) {
		t.Parallel()

		nonSideloadDeployment := validConnection
		nonSideloadDeployment.Deployment.Type = ArgoManifestsDeploymentType
		serializedConnection, marshalErr := json.Marshal(nonSideloadDeployment)
		require.NoError(t, marshalErr)

		theCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      connectionsConfigmapName,
				Namespace: defaultNamespace,
				Labels: map[string]string{
					kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
				},
			},
			Data: map[string]string{
				"argo-test": string(serializedConnection),
			},
		}

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theCM, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, WithK8sClient(fake.NewClientset(theCM)))
		require.NoError(t, octantConnection.DeleteConnection(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}))
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "argo-test").
			Return(argoIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Once()
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "argo-test", mock.Anything, false).
			Return(nil).
			Once()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(
			mockCmStore,
			testConfig,
			WithK8sClient(fake.NewClientset()),
			WithArgoCDIntegration(mockArgoIntegration),
			WithArgoClient(mockArgoClient),
		)
		require.NoError(t, octantConnection.DeleteConnection(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}))
	})
}

func TestGetConnectionStatus(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType:     "datadog",
		TelemetryTypes: telemetryTypes,
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
		MdaiNamespace: defaultNamespace,
		Destinations: []OctantConnectionDestination{
			{
				DestinationType: "datadog",
				IntegrationName: "argo-test",
			},
		},
	}

	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	theConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      connectionsConfigmapName,
			Namespace: defaultNamespace,
			Labels: map[string]string{
				kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
			},
		},
		Data: map[string]string{
			"argo-test": string(validConnectionBytes),
		},
	}

	validatorRunID := "abc-xyz-123"

	t.Run("configmap not found", func(t *testing.T) {
		t.Parallel()

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, connectionsConfigmapName)
		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(nil, notFoundError).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		status, getErr := octantConnection.GetConnectionStatus(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}, validatorRunID)
		require.ErrorContains(t, getErr, "connection 'argo-test' not found in namespace")
		require.Nil(t, status)
	})

	t.Run("connection not found", func(t *testing.T) {
		t.Parallel()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		status, getErr := octantConnection.GetConnectionStatus(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test-yolo",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}, validatorRunID)
		require.ErrorContains(t, getErr, "getting connection")
		require.Nil(t, status)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		theResponse := &octantv1alpha.GetConnectionStatusResponse{
			ReceivingData:    true,
			SendingData:      false,
			DataIntegrity:    false,
			ClientsConnected: true,
		}
		mockConnectionStatus := metricsmock.NewMockConnectionStatus(t)
		mockConnectionStatus.EXPECT().
			GetConnectionStatus(mock.Anything, defaultNamespace, "argo-test", telemetryTypes, validatorRunID).
			Return(theResponse, nil).
			Once()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, WithConnectionMetrics(mockConnectionStatus))
		status, getErr := octantConnection.GetConnectionStatus(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}, validatorRunID)
		require.NoError(t, getErr)
		require.NotNil(t, status)
	})
}

func TestPutConnectionValidatorRun(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType:     "datadog",
		TelemetryTypes: telemetryTypes,
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
		MdaiNamespace: defaultNamespace,
		Destinations: []OctantConnectionDestination{
			{
				DestinationType: "datadog",
				IntegrationName: "argo-test",
			},
		},
	}

	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	theConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      connectionsConfigmapName,
			Namespace: defaultNamespace,
			Labels: map[string]string{
				kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
			},
		},
		Data: map[string]string{
			"argo-test": string(validConnectionBytes),
		},
	}

	t.Run("configmap not found", func(t *testing.T) {
		t.Parallel()

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, connectionsConfigmapName)
		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(nil, notFoundError).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		runID, getErr := octantConnection.PutConnectionValidatorRun(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "connection 'argo-test' not found")
		require.Empty(t, runID)
	})

	t.Run("connection not found", func(t *testing.T) {
		t.Parallel()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		runID, getErr := octantConnection.PutConnectionValidatorRun(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test-yolo",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "getting connection")
		require.Empty(t, runID)
	})

	t.Run("happy path - non-sideload deployment skips execution", func(t *testing.T) {
		t.Parallel()

		nonSideloadDeployment := validConnection
		nonSideloadDeployment.Deployment.Type = ArgoManifestsDeploymentType
		serializedConnection, marshalErr := json.Marshal(nonSideloadDeployment)
		require.NoError(t, marshalErr)
		theCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      connectionsConfigmapName,
				Namespace: defaultNamespace,
				Labels: map[string]string{
					kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
				},
			},
			Data: map[string]string{
				"argo-test": string(serializedConnection),
			},
		}

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theCM, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig)
		runID, getErr := octantConnection.PutConnectionValidatorRun(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
		require.Empty(t, runID) // empty string return when we don't sideload deployment
	})

	t.Run("happy path - with sideload validator deployment", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "argo-test").
			Return(argoIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "argo-test", mock.Anything, false).
			Return(nil).
			Once()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(
			mockCmStore,
			testConfig,
			WithArgoCDIntegration(mockArgoIntegration),
			WithArgoClient(mockArgoClient),
			WithGenerator(NewConnectionManifestGenerator(testConfig)),
		)
		runID, getErr := octantConnection.PutConnectionValidatorRun(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
		require.NotEmpty(t, runID) // non-empty validator runID returned
	})
}

func TestDeleteConnectionValidator(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType:     "datadog",
		TelemetryTypes: telemetryTypes,
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
		MdaiNamespace: defaultNamespace,
		Destinations: []OctantConnectionDestination{
			{
				DestinationType: "datadog",
				IntegrationName: "argo-test",
			},
		},
	}

	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)
	mockK8sData := []runtime.Object{
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
			Data: map[string]string{
				"argo-test": string(validConnectionBytes),
			},
		},
	}

	t.Run("configmap not found", func(t *testing.T) {
		t.Parallel()

		octantConnection := NewOctantConnection(fake.NewClientset(), nil, nil, nil, testConfig, nil, nil)
		getErr := octantConnection.DeleteConnectionValidator(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "failed to fetch configmap")
	})

	t.Run("connection not found", func(t *testing.T) {
		t.Parallel()

		octantConnection := NewOctantConnection(fake.NewClientset(mockK8sData...), nil, nil, nil, testConfig, nil, nil)
		getErr := octantConnection.DeleteConnectionValidator(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test-yolo",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "connection not found in configmap")
	})

	t.Run("error unmarshalling connection data", func(t *testing.T) {
		t.Parallel()

		k8sData := []runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
				Data: map[string]string{
					"argo-test": "}invalid{json",
				},
			},
		}

		generator := NewConnectionManifestGenerator(testConfig)
		octantConnection := NewOctantConnection(fake.NewClientset(k8sData...), nil, nil, nil, testConfig, nil, generator)
		getErr := octantConnection.DeleteConnectionValidator(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "failed to unmarshal connection data")
	})

	t.Run("happy path - non-sideload deployment skips execution", func(t *testing.T) {
		t.Parallel()

		nonSideloadDeployment := validConnection
		nonSideloadDeployment.Deployment.Type = ArgoManifestsDeploymentType
		serializedConnection, marshalErr := json.Marshal(nonSideloadDeployment)
		require.NoError(t, marshalErr)
		k8sData := []runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
				Data: map[string]string{
					"argo-test": string(serializedConnection),
				},
			},
		}

		generator := NewConnectionManifestGenerator(testConfig)
		octantConnection := NewOctantConnection(fake.NewClientset(k8sData...), nil, nil, nil, testConfig, nil, generator)
		getErr := octantConnection.DeleteConnectionValidator(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
	})

	t.Run("happy path - with sideload validator deployment", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "argo-test").
			Return(argoIntegrationData, nil).
			Once()

		mockDatadogIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadogIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "argo-test").
			Return(ddIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "argo-test", mock.Anything, true).
			Return(nil).
			Once()

		generator := NewConnectionManifestGenerator(testConfig)
		octantConnection := NewOctantConnection(fake.NewClientset(mockK8sData...), mockArgoIntegration, mockDatadogIntegration, nil, testConfig, mockArgoClient, generator)
		getErr := octantConnection.DeleteConnectionValidator(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
	})
}

func TestGetConnections(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType:     "datadog",
		TelemetryTypes: telemetryTypes,
		Deployment: &Deployment{
			Type:            ArgoSideloadDeploymentType,
			IntegrationName: "argo-test",
		},
		MdaiNamespace: defaultNamespace,
		Destinations: []OctantConnectionDestination{
			{
				DestinationType: "datadog",
				IntegrationName: "argo-test",
			},
		},
	}

	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)
	mockK8sData := []runtime.Object{
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
			Data: map[string]string{
				"argo-test": string(validConnectionBytes),
			},
		},
	}

	t.Run("configmap not found", func(t *testing.T) {
		t.Parallel()

		octantConnection := NewOctantConnection(fake.NewClientset(), nil, nil, nil, testConfig, nil, nil)
		connections, getErr := octantConnection.GetConnections(t.Context(), ConnectionCRUDInput{
			Logger: zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "failed to get configmap")
		require.Nil(t, connections)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		octantConnection := NewOctantConnection(fake.NewClientset(mockK8sData...), nil, nil, nil, testConfig, nil, nil)
		connections, getErr := octantConnection.GetConnections(t.Context(), ConnectionCRUDInput{
			Logger: zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
		require.Len(t, connections, 1)
		assert.Equal(t, "argo-test", connections[0])
	})
}
