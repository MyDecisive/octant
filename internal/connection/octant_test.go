package connection

import (
	"encoding/json"
	"testing"

	"github.com/mydecisive/mdai-gateway/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const defaultNamespace = "default"

func TestGetConnectionByName(t *testing.T) {
	t.Parallel()

	validConnection := OctantConnectionData{
		SourceType: "datadog",
		TelemetryTypes: []telemetry.MLT{
			telemetry.Logs,
			telemetry.Traces,
		},
		Deployment: &Deployment{
			Type: "argocd",
			Fields: map[string]any{
				"branch": "tv/bestBranch",
			},
		},
	}
	validConnectionBytes, err := json.Marshal(validConnection)
	require.NoError(t, err)

	t.Run("configmap does not exist", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		actual, getErr := octantConnection.GetConnectionByName(t.Context(), defaultNamespace, "doesntMatter")
		require.NoError(t, getErr)
		require.Nil(t, actual)
	})

	t.Run("connection not found", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
				Data: map[string]string{
					"team-a": string(validConnectionBytes),
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		actual, getErr := octantConnection.GetConnectionByName(t.Context(), defaultNamespace, "team-b")
		require.NoError(t, getErr)
		require.Nil(t, actual)
	})

	t.Run("connection unmarshal error", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
				Data: map[string]string{
					"team-a": "not gonna work",
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		actual, getErr := octantConnection.GetConnectionByName(t.Context(), defaultNamespace, "team-a")
		require.ErrorContains(t, getErr, "failed to unmarshal connection data")
		require.Nil(t, actual)
	})

	t.Run("configmap exists with valid connections", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
				Data: map[string]string{
					"team-a": string(validConnectionBytes),
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		actual, getErr := octantConnection.GetConnectionByName(t.Context(), defaultNamespace, "team-a")
		require.NoError(t, getErr)

		require.True(t, assert.ObjectsAreEqual(&validConnection, actual), "expected and actual don't match")
	})
}

func TestArgoCD_SetIntegration(t *testing.T) {
	t.Parallel()

	newConnection := OctantConnectionData{
		SourceType: "datadog",
		TelemetryTypes: []telemetry.MLT{
			telemetry.Logs,
			telemetry.Traces,
		},
		Deployment: &Deployment{
			Type: "argocd",
			Fields: map[string]any{
				"branch": "tv/bestBranch",
			},
		},
	}

	t.Run("creates configmap when it does not exist", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		// Verify the configmap doesn't exist yet
		_, err := mockK8sClient.CoreV1().ConfigMaps(defaultNamespace).Get(t.Context(), connectionsConfigmapName, metav1.GetOptions{})
		require.ErrorContains(t, err, "configmaps \"mdai-octant-connections\" not found")

		err = octantConnection.SaveConnection(t.Context(), newConnection, defaultNamespace, "team-a")
		require.NoError(t, err)

		// Verify the configmap actually contains the added integration
		secret, getErr := mockK8sClient.CoreV1().ConfigMaps(defaultNamespace).Get(t.Context(), connectionsConfigmapName, metav1.GetOptions{})
		require.NoError(t, getErr)
		require.NotNil(t, secret.Data)
		require.Len(t, secret.Data, 1)
		require.Contains(t, secret.Data, "team-a")

		var teamData OctantConnectionData
		err = json.Unmarshal([]byte(secret.Data["team-a"]), &teamData)
		require.NoError(t, err)

		assert.Equal(t, newConnection, teamData)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
				Data: map[string]string{
					"team-a": `{"sourceType": "datadog", "telemetryTypes": ["logs", "traces"], "deployment": {"type": "argocd", "fields": {"branch": "tv/coolBranch"}}}`,
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		// Verify the secret DOES exist already
		existingConfigmap, err := mockK8sClient.CoreV1().ConfigMaps(defaultNamespace).Get(t.Context(), connectionsConfigmapName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingConfigmap.Data)
		require.Len(t, existingConfigmap.Data, 1)
		require.Contains(t, existingConfigmap.Data, "team-a")

		err = octantConnection.SaveConnection(t.Context(), newConnection, defaultNamespace, "team-b")
		require.NoError(t, err)

		// Verify the secret actually contains the added integration
		secret, getErr := mockK8sClient.CoreV1().ConfigMaps(defaultNamespace).Get(t.Context(), connectionsConfigmapName, metav1.GetOptions{})
		require.NoError(t, getErr)
		require.NotNil(t, secret.Data)
		require.Len(t, secret.Data, 2)
		require.Contains(t, secret.Data, "team-b")

		var teamData OctantConnectionData
		err = json.Unmarshal([]byte(secret.Data["team-b"]), &teamData)
		require.NoError(t, err)

		assert.Equal(t, newConnection, teamData)
	})
}

func TestArgoCD_DeleteIntegration(t *testing.T) {
	t.Parallel()

	t.Run("secret does not exist - silently succeeds", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		// validate configmap doesn't exist before we try to delete
		_, err := mockK8sClient.CoreV1().ConfigMaps(defaultNamespace).Get(t.Context(), connectionsConfigmapName, metav1.GetOptions{})
		require.ErrorContains(t, err, "configmaps \"mdai-octant-connections\" not found")

		err = octantConnection.DeleteConnection(t.Context(), defaultNamespace, "team-a")
		require.NoError(t, err)
	})

	t.Run("connection does not exist in configmap - silently succeeds", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
				Data: map[string]string{
					"team-a": `{"sourceType": "datadog", "telemetryTypes": ["logs", "traces"], "deployment": {"type": "argocd", "fields": {"branch": "tv/coolBranch"}}}`,
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		// validate configmap exists with "team-a" before we try to delete with another connection name
		existingConfigmap, err := mockK8sClient.CoreV1().ConfigMaps(defaultNamespace).Get(t.Context(), connectionsConfigmapName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingConfigmap.Data)
		require.Len(t, existingConfigmap.Data, 1)
		require.Contains(t, existingConfigmap.Data, "team-a")

		err = octantConnection.DeleteConnection(t.Context(), defaultNamespace, "team-b")
		require.NoError(t, err)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: connectionsConfigmapName, Namespace: defaultNamespace},
				Data: map[string]string{
					"team-a": `{"sourceType": "datadog", "telemetryTypes": ["logs", "traces"], "deployment": {"type": "argocd", "fields": {"branch": "tv/coolBranch"}}}`,
					"team-b": `{"sourceType": "datadog", "telemetryTypes": ["metrics"], "deployment": {"type": "argocd", "fields": {"branch": "main"}}}`,
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		octantConnection := NewOctantConnection(mockK8sClient, nil, nil)

		// validate secret exists with both integration names before we delete one of them.
		existingConfigmap, err := mockK8sClient.CoreV1().ConfigMaps(defaultNamespace).Get(t.Context(), connectionsConfigmapName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingConfigmap.Data)
		require.Len(t, existingConfigmap.Data, 2)
		require.Contains(t, existingConfigmap.Data, "team-a")
		require.Contains(t, existingConfigmap.Data, "team-b")

		err = octantConnection.DeleteConnection(t.Context(), defaultNamespace, "team-a")
		require.NoError(t, err)

		existingConfigmap, err = mockK8sClient.CoreV1().ConfigMaps(defaultNamespace).Get(t.Context(), connectionsConfigmapName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingConfigmap.Data)
		require.Len(t, existingConfigmap.Data, 1)
		require.NotContains(t, existingConfigmap.Data, "team-a") // team-a was deleted
		require.Contains(t, existingConfigmap.Data, "team-b")
	})
}
