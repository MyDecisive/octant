package integration

import (
	"encoding/json"
	"testing"

	"github.com/mydecisive/mdai-data-core/kube"
	kubemock "github.com/mydecisive/mdai-data-core/mock/kube"
	"github.com/mydecisive/octant/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
)

func TestArgoCD_GetIntegrations(t *testing.T) {
	t.Parallel()

	secretMeta := metav1.ObjectMeta{
		Name:      argocdSecretName,
		Namespace: defaultNamespace,
		Labels: map[string]string{
			kube.SecretTypeLabel: kube.OctantIntegrationArgoType,
		},
	}
	validInt := ArgoCDIntegrationData{
		AccountToken: "abc123",
		APIUrl:       "http://localhost:12345",
	}
	validIntBytes, err := json.Marshal(validInt)
	require.NoError(t, err)

	t.Run("secret does not exist", func(t *testing.T) {
		t.Parallel()

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, argocdSecretName)
		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(nil, notFoundError).
			Once()
		argocdIntegration := &ArgoCDIntegration{
			SecretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
		}

		actual, getErr := argocdIntegration.GetIntegrations(t.Context())
		require.NoError(t, getErr)
		require.Nil(t, actual)
	})

	t.Run("secret exists with valid integrations", func(t *testing.T) {
		t.Parallel()

		existingSecret := &corev1.Secret{
			ObjectMeta: secretMeta,
			Data: map[string][]byte{
				"team-a": validIntBytes,
			},
		}

		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(existingSecret, nil).
			Once()
		argocdIntegration := &ArgoCDIntegration{
			SecretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
		}

		actual, getErr := argocdIntegration.GetIntegrations(t.Context())
		require.NoError(t, getErr)

		require.True(t, assert.ObjectsAreEqual(map[string]ArgoCDIntegrationData{
			"team-a": validInt,
		}, actual), "expected and actual don't match")
	})

	t.Run("secret exists with invalid json skips the bad entry", func(t *testing.T) {
		t.Parallel()

		existingSecret := &corev1.Secret{
			ObjectMeta: secretMeta,
			Data: map[string][]byte{
				"team-a": validIntBytes,
				"team-b": []byte("invalid-json"),
			},
		}

		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(existingSecret, nil).
			Once()
		argocdIntegration := &ArgoCDIntegration{
			SecretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
		}

		actual, getErr := argocdIntegration.GetIntegrations(t.Context())
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

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, argocdSecretName)
		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(nil, notFoundError).
			Once()
		argocdIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			SecretStore: secretStore,
		}

		err := argocdIntegration.SetIntegration(t.Context(), "team-a", newIntegration)
		require.NoError(t, err)

		// Verify the secret actually contains the added integration
		secret, getErr := mockK8sClient.CoreV1().
			Secrets(defaultNamespace).
			Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.NoError(t, getErr)
		require.NotNil(t, secret.Data)
		require.Len(t, secret.Data, 1)
		require.Contains(t, secret.Data, "team-a")
		require.Contains(t, secret.Labels, kube.SecretTypeLabel)
		assert.Equal(t, kube.OctantIntegrationArgoType, secret.Labels[kube.SecretTypeLabel])

		var teamData ArgoCDIntegrationData
		err = json.Unmarshal(secret.Data["team-a"], &teamData)
		require.NoError(t, err)

		assert.Equal(t, newIntegration, teamData)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      argocdSecretName,
				Namespace: defaultNamespace,
				Labels: map[string]string{
					kube.SecretTypeLabel: kube.OctantIntegrationArgoType,
				},
			},
			Data: map[string][]byte{
				"team-a": []byte(`{"accountToken":"old-account-token", "apiUrl":"http://localhost:12345"}`),
			},
		}
		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(existingSecret, nil).
			Once()
		mockK8sClient := fake.NewClientset(existingSecret)
		datadogIntegration := &ArgoCDIntegration{
			K8sClient: mockK8sClient,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			SecretStore: secretStore,
		}

		// Verify the secret DOES exist already
		existingSecret, err := mockK8sClient.CoreV1().
			Secrets(defaultNamespace).
			Get(t.Context(), argocdSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 1)
		require.Contains(t, existingSecret.Data, "team-a")

		err = datadogIntegration.SetIntegration(t.Context(), "team-b", newIntegration)
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

	secretMeta := metav1.ObjectMeta{
		Name:      argocdSecretName,
		Namespace: defaultNamespace,
		Labels: map[string]string{
			kube.SecretTypeLabel: kube.OctantIntegrationArgoType,
		},
	}

	t.Run("secret does not exist - silently succeeds", func(t *testing.T) {
		t.Parallel()

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, argocdSecretName)
		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(nil, notFoundError).
			Once()
		datadogIntegration := &ArgoCDIntegration{
			SecretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
		}

		err := datadogIntegration.DeleteIntegration(t.Context(), "team-a")
		require.NoError(t, err)
	})

	t.Run("integration does not exist in secret - silently succeeds", func(t *testing.T) {
		t.Parallel()

		existingSecret := &corev1.Secret{
			ObjectMeta: secretMeta,
			Data: map[string][]byte{
				"team-a": []byte(`{"accountToken":"abc123-token", "apiUrl":"http://localhost:12345"}`),
			},
		}
		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(existingSecret, nil).
			Once()
		argocdIntegration := &ArgoCDIntegration{
			SecretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
		}

		err := argocdIntegration.DeleteIntegration(t.Context(), "team-b")
		require.NoError(t, err)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingSecret := &corev1.Secret{
			ObjectMeta: secretMeta,
			Data: map[string][]byte{
				"team-a": []byte(`{"accountToken":"abc123-token", "apiUrl": "http://localhost:12345"}`),
				"team-b": []byte(`{"accountToken":"xyz999-token", "apiUrl": "http://localhost:12345"}`),
			},
		}

		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(existingSecret, nil).
			Once()
		mockK8sClient := fake.NewClientset(existingSecret)
		argocdIntegration := &ArgoCDIntegration{
			K8sClient:   mockK8sClient,
			SecretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
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

		err = argocdIntegration.DeleteIntegration(t.Context(), "team-a")
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
