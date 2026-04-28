package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestArgoCD_GetIntegrations(t *testing.T) {
	t.Parallel()

	validInt := ArgoCDIntegrationData{
		AccountToken: "abc123",
		APIUrl:       "http://localhost:12345",
	}
	validIntBytes, err := json.Marshal(validInt)
	require.NoError(t, err)

	t.Run("secret does not exist", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		argocdIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
		}

		actual, getErr := argocdIntegration.GetIntegrations(t.Context(), defaultNamespace)
		require.NoError(t, getErr)
		require.Nil(t, actual)
	})

	t.Run("secret exists with valid integrations", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: argocdSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": validIntBytes,
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		argocdIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
		}

		actual, getErr := argocdIntegration.GetIntegrations(t.Context(), defaultNamespace)
		require.NoError(t, getErr)

		require.True(t, assert.ObjectsAreEqual(map[string]ArgoCDIntegrationData{
			"team-a": validInt,
		}, actual), "expected and actual don't match")
	})

	t.Run("secret exists with invalid json skips the bad entry", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: argocdSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": validIntBytes,
					"team-b": []byte("invalid-json"),
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		argocdIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
		}

		actual, getErr := argocdIntegration.GetIntegrations(t.Context(), defaultNamespace)
		require.NoError(t, getErr)
		require.True(t, assert.ObjectsAreEqual(map[string]ArgoCDIntegrationData{
			"team-a": validInt,
		}, actual), "expected and actual don't match")
	})
}

func TestArgoCD_SetIntegration(t *testing.T) {
	t.Parallel()

	newIntegration := ArgoCDIntegrationData{
		AccountToken: "abc123",
		APIUrl:       "http://localhost:12345",
	}

	t.Run("creates secret when it does not exist", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		argocdIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
		}

		// Verify the secret doesn't exist yet
		_, err := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.ErrorContains(t, err, "secrets \"mdai-argocd-integration\" not found")

		err = argocdIntegration.SetIntegration(t.Context(), defaultNamespace, "team-a", newIntegration)
		require.NoError(t, err)

		// Verify the secret actually contains the added integration
		secret, getErr := mockK8sClient.CoreV1().
			Secrets(defaultNamespace).
			Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.NoError(t, getErr)
		require.NotNil(t, secret.Data)
		require.Len(t, secret.Data, 1)
		require.Contains(t, secret.Data, "team-a")

		var teamData ArgoCDIntegrationData
		err = json.Unmarshal(secret.Data["team-a"], &teamData)
		require.NoError(t, err)

		assert.Equal(t, newIntegration, teamData)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: argocdSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": []byte(`{"accountToken":"old-account-token", "apiUrl":"http://localhost:12345"}`),
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		datadogIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
		}

		// Verify the secret DOES exist already
		existingSecret, err := mockK8sClient.CoreV1().
			Secrets(defaultNamespace).
			Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 1)
		require.Contains(t, existingSecret.Data, "team-a")

		err = datadogIntegration.SetIntegration(t.Context(), defaultNamespace, "team-b", newIntegration)
		require.NoError(t, err)

		// Verify the secret actually contains the added integration
		secret, getErr := mockK8sClient.CoreV1().
			Secrets(defaultNamespace).
			Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.NoError(t, getErr)
		require.NotNil(t, secret.Data)
		require.Len(t, secret.Data, 2)
		require.Contains(t, secret.Data, "team-b")

		var teamData ArgoCDIntegrationData
		err = json.Unmarshal(secret.Data["team-b"], &teamData)
		require.NoError(t, err)

		assert.Equal(t, newIntegration, teamData)
	})
}

func TestArgoCD_DeleteIntegration(t *testing.T) {
	t.Parallel()

	t.Run("secret does not exist - silently succeeds", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		datadogIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
		}

		// validate secret doesn't exist before we try to delete
		_, err := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.ErrorContains(t, err, "secrets \"mdai-argocd-integration\" not found")

		err = datadogIntegration.DeleteIntegration(t.Context(), defaultNamespace, "team-a")
		require.NoError(t, err)
	})

	t.Run("integration does not exist in secret - silently succeeds", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: argocdSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": []byte(`{"accountToken":"abc123-token", "apiUrl":"http://localhost:12345"}`),
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		argocdIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
		}

		// validate secret exists with "team-a" before we try to delete with another integration name
		existingSecret, err := mockK8sClient.CoreV1().
			Secrets(defaultNamespace).
			Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 1)
		require.Contains(t, existingSecret.Data, "team-a")

		err = argocdIntegration.DeleteIntegration(t.Context(), defaultNamespace, "team-b")
		require.NoError(t, err)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: argocdSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": []byte(`{"accountToken":"abc123-token", "apiUrl": "http://localhost:12345"}`),
					"team-b": []byte(`{"accountToken":"xyz999-token", "apiUrl": "http://localhost:12345"}`),
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		argocdIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
		}

		// validate secret exists with both integration names before we delete one of them.
		existingSecret, err := mockK8sClient.CoreV1().
			Secrets(defaultNamespace).
			Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 2)
		require.Contains(t, existingSecret.Data, "team-a")
		require.Contains(t, existingSecret.Data, "team-b")

		err = argocdIntegration.DeleteIntegration(t.Context(), defaultNamespace, "team-a")
		require.NoError(t, err)

		existingSecret, err = mockK8sClient.CoreV1().
			Secrets(defaultNamespace).
			Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 1)
		require.NotContains(t, existingSecret.Data, "team-a") // team-a was deleted
		require.Contains(t, existingSecret.Data, "team-b")
	})
}
