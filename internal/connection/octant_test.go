package connection

import (
	"encoding/json"
	"testing"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/mdai-data-core/kube"
	kubemock "github.com/mydecisive/mdai-data-core/mock/kube"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection/manifest"
	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/mydecisive/octant/internal/integration"
	manifestmock "github.com/mydecisive/octant/internal/mock/manifest"
	metricsmock "github.com/mydecisive/octant/internal/mock/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		Install: config.Install{
			MdaiValidatorVersion: "0.1.3",
		},
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		input := ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}

		// set the Created timestamp to compare after the update
		now := time.Now()
		newConnection := validConnection
		newConnection.Created = now
		validConnectionBytes, err := json.Marshal(newConnection)
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

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()
		mockCmStore.EXPECT().
			UpdateConfigMap(mock.Anything, testConfig.CurrentNamespace, mock.MatchedBy(func(cm *corev1.ConfigMap) bool {
				// verify the Created timestamp didn't get changed
				var updatedConnection OctantConnectionData
				err = json.Unmarshal([]byte(cm.Data["argo-test"]), &updatedConnection)
				require.NoError(t, err)
				assert.True(t, updatedConnection.Created.Equal(now), "expected created time to equal now")

				return cm.Name == connectionsConfigmapName
			})).
			Return(nil).
			Once()

		mockManager := manifestmock.NewMockManager(t)
		mockManager.EXPECT().LoadConnection(mock.Anything, mock.Anything, mock.MatchedBy(func(in manifestdata.ConnectionInput) bool {
			return !in.Exported && in.DeploymentIntegrationName == validConnection.Deployment.IntegrationName &&
				in.ConnectionName == input.ConnectionName && in.Namespace == input.Namespace
		})).Return(nil).Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, mockManager)
		require.NoError(t, octantConnection.SaveConnection(t.Context(), validConnection, input))
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		input := ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, connectionsConfigmapName)
		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(nil, notFoundError).
			Once()
		mockCmStore.EXPECT().
			CreateConfigMap(mock.Anything, testConfig.CurrentNamespace, mock.MatchedBy(func(cm *corev1.ConfigMap) bool {
				return cm.Name == connectionsConfigmapName
			})).
			Return(nil).
			Once()

		mockManager := manifestmock.NewMockManager(t)
		mockManager.EXPECT().LoadConnection(mock.Anything, mock.Anything, mock.MatchedBy(func(in manifestdata.ConnectionInput) bool {
			return !in.Exported && in.DeploymentIntegrationName == validConnection.Deployment.IntegrationName &&
				in.ConnectionName == input.ConnectionName && in.Namespace == input.Namespace
		})).Return(nil).Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, mockManager)
		require.NoError(t, octantConnection.SaveConnection(t.Context(), validConnection, input))
	})

	t.Run("success skip & no deploy", func(t *testing.T) {
		t.Parallel()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockManager := manifestmock.NewMockManager(t)

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, mockManager)
		require.NoError(t, octantConnection.SaveConnection(t.Context(), validConnection, ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
			OnlyDeploy:     true,
			NoDeploy:       true,
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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
		mockCmStore.EXPECT().
			UpdateConfigMap(mock.Anything, testConfig.CurrentNamespace, mock.MatchedBy(func(cm *corev1.ConfigMap) bool {
				return cm.Name == connectionsConfigmapName
			})).
			Return(nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
		require.NoError(t, octantConnection.DeleteConnection(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}))
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		input := ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()
		mockCmStore.EXPECT().
			UpdateConfigMap(mock.Anything, testConfig.CurrentNamespace, mock.MatchedBy(func(cm *corev1.ConfigMap) bool {
				return cm.Name == connectionsConfigmapName
			})).
			Return(nil).
			Once()

		mockManager := manifestmock.NewMockManager(t)
		mockManager.EXPECT().Unload(mock.Anything, mock.MatchedBy(func(in manifest.ManagerInput) bool {
			return in.ConnectionName == input.ConnectionName &&
				in.DeploymentIntegrationName == validConnection.Deployment.IntegrationName
		}), []manifestdata.App{manifestdata.CONNECTION, manifestdata.VALIDATOR}).Return(nil).Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, mockManager)
		require.NoError(t, octantConnection.DeleteConnection(t.Context(), input))
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, mockConnectionStatus, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		input := ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		mockManager := manifestmock.NewMockManager(t)
		mockManager.EXPECT().LoadValidator(mock.Anything, mock.Anything, mock.MatchedBy(func(in manifestdata.ValidatorInput) bool {
			return in.DeploymentIntegrationName == validConnection.Deployment.IntegrationName &&
				in.ConnectionName == input.ConnectionName && in.Namespace == input.Namespace
		})).Return(nil).Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, mockManager)
		runID, getErr := octantConnection.PutConnectionValidatorRun(t.Context(), input)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
		getErr := octantConnection.DeleteConnectionValidator(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "failed to fetch configmap")
	})

	t.Run("connection not found", func(t *testing.T) {
		t.Parallel()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
		getErr := octantConnection.DeleteConnectionValidator(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test-yolo",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "connection not found in configmap")
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
		getErr := octantConnection.DeleteConnectionValidator(t.Context(), ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
	})

	t.Run("happy path - with sideload validator deployment", func(t *testing.T) {
		t.Parallel()

		input := ConnectionCRUDInput{
			ConnectionName: "argo-test",
			Namespace:      defaultNamespace,
			Logger:         zaptest.NewLogger(t),
		}

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		mockManager := manifestmock.NewMockManager(t)
		mockManager.EXPECT().Unload(mock.Anything, mock.MatchedBy(func(in manifest.ManagerInput) bool {
			return in.ConnectionName == input.ConnectionName &&
				in.DeploymentIntegrationName == validConnection.Deployment.IntegrationName
		}), []manifestdata.App{manifestdata.VALIDATOR}).Return(nil).Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, mockManager)

		getErr := octantConnection.DeleteConnectionValidator(t.Context(), input)
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

	theConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      connectionsConfigmapName,
			Namespace: defaultNamespace,
			Labels: map[string]string{
				kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
			},
		},
		Data: map[string]string{
			"argo-test-1": string(validConnectionBytes),
			"argo-test-2": string(validConnectionBytes),
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

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
		connections, getErr := octantConnection.GetConnections(t.Context(), ConnectionCRUDInput{
			Logger: zaptest.NewLogger(t),
		})
		require.ErrorContains(t, getErr, "failed to get configmap")
		require.Nil(t, connections)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockCmStore := kubemock.NewMockConfigMapStore(t)
		mockCmStore.EXPECT().
			GetConfigmapByNameAndNamespace(connectionsConfigmapName, testConfig.CurrentNamespace).
			Return(theConfigmap, nil).
			Once()

		octantConnection := NewOctantConnection(mockCmStore, testConfig, nil, nil)
		connections, getErr := octantConnection.GetConnections(t.Context(), ConnectionCRUDInput{
			Logger: zaptest.NewLogger(t),
		})
		require.NoError(t, getErr)
		require.Len(t, connections, 2)
		assert.ElementsMatch(t, []string{"argo-test-1", "argo-test-2"}, connections)
	})
}
