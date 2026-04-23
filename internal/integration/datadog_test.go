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

const defaultNamespace = "default"

func TestGetIntegrations(t *testing.T) {
	t.Parallel()

	validInt := DataDogIntegrationData{APIKey: "12345", DDUrl: "https://example.com"}
	validIntBytes, err := json.Marshal(validInt)
	require.NoError(t, err)

	t.Run("secret does not exist", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		actual, err := datadogIntegration.GetIntegrations(t.Context(), defaultNamespace)
		require.NoError(t, err)
		require.Nil(t, actual)
	})

	t.Run("secret exists with valid integrations", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: datadogSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": validIntBytes,
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		actual, err := datadogIntegration.GetIntegrations(t.Context(), defaultNamespace)
		require.NoError(t, err)

		require.True(t, assert.ObjectsAreEqual(map[string]DataDogIntegrationData{
			"team-a": validInt,
		}, actual), "expected and actual don't match")
	})

	t.Run("secret exists with invalid json skips the bad entry", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: datadogSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": validIntBytes,
					"team-b": []byte("invalid-json"),
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		actual, err := datadogIntegration.GetIntegrations(t.Context(), defaultNamespace)
		require.NoError(t, err)
		require.True(t, assert.ObjectsAreEqual(map[string]DataDogIntegrationData{
			"team-a": validInt,
		}, actual), "expected and actual don't match")
	})
}

func TestGetIntegrationByName(t *testing.T) {
	t.Parallel()

	validInt := DataDogIntegrationData{APIKey: "12345", DDUrl: "https://example.com"}
	validIntBytes, err := json.Marshal(validInt)
	require.NoError(t, err)

	t.Run("secret does not exist", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		actual, getErr := datadogIntegration.GetIntegrationByName(t.Context(), defaultNamespace, "doesntMatter")
		require.NoError(t, getErr)
		require.Nil(t, actual)
	})

	t.Run("integration not found", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: datadogSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": validIntBytes,
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		actual, getErr := datadogIntegration.GetIntegrationByName(t.Context(), defaultNamespace, "team-b")
		require.ErrorContains(t, getErr, "integration 'team-b' not found")
		require.Nil(t, actual)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: datadogSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": validIntBytes,
				},
			},
		}

		mockK8sClient := fake.NewClientset(existingObjects...)
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		actual, getErr := datadogIntegration.GetIntegrationByName(t.Context(), defaultNamespace, "team-a")
		require.NoError(t, getErr)

		require.True(t, assert.ObjectsAreEqual(&validInt, actual), "expected and actual don't match")
	})
}

func TestSetIntegration(t *testing.T) {
	t.Parallel()

	newIntegration := DataDogIntegrationData{APIKey: "new-key", DDUrl: "https://example.com"}

	t.Run("creates secret when it does not exist", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		// Verify the secret doesn't exist yet
		_, err := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), datadogSecretName, metav1.GetOptions{})
		require.ErrorContains(t, err, "secrets \"mdai-datadog-integration\" not found")

		err = datadogIntegration.SetIntegration(t.Context(), defaultNamespace, "doesntMatter", newIntegration)
		require.NoError(t, err)

		// Verify the secret actually contains the added integration
		secret, getErr := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), datadogSecretName, metav1.GetOptions{})
		require.NoError(t, getErr)
		require.NotNil(t, secret.Data)
		require.Len(t, secret.Data, 1)
		require.Contains(t, secret.Data, datadogSecretName)

		var teamData DataDogIntegrationData
		err = json.Unmarshal(secret.Data[datadogSecretName], &teamData)
		require.NoError(t, err)

		assert.Equal(t, newIntegration, teamData)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: datadogSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					datadogSecretName: []byte(`{"api_key":"old-key","dd_url":"old-url"}`),
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		// Verify the secret DOES exist already
		existingSecret, err := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), datadogSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 1)
		require.Contains(t, existingSecret.Data, datadogSecretName)

		err = datadogIntegration.SetIntegration(t.Context(), defaultNamespace, "doesntMatter", newIntegration)
		require.NoError(t, err)

		// Verify the secret actually contains the added integration
		secret, getErr := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), datadogSecretName, metav1.GetOptions{})
		require.NoError(t, getErr)
		require.NotNil(t, secret.Data)
		require.Len(t, secret.Data, 1)
		require.Contains(t, secret.Data, datadogSecretName)

		var teamData DataDogIntegrationData
		err = json.Unmarshal(secret.Data[datadogSecretName], &teamData)
		require.NoError(t, err)

		assert.Equal(t, newIntegration, teamData)
	})
}

func TestDeleteIntegration(t *testing.T) {
	t.Parallel()

	t.Run("secret does not exist - silently succeeds", func(t *testing.T) {
		t.Parallel()

		mockK8sClient := fake.NewClientset()
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		// validate secret doesn't exist before we try to delete
		_, err := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), datadogSecretName, metav1.GetOptions{})
		require.ErrorContains(t, err, "secrets \"mdai-datadog-integration\" not found")

		err = datadogIntegration.DeleteIntegration(t.Context(), defaultNamespace, "team-a")
		require.NoError(t, err)
	})

	t.Run("integration does not exist in secret - silently succeeds", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: datadogSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": []byte(`{"api_key":"key","dd_url":"url"}`),
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		// validate secret exists with "team-a" before we try to delete with another integration name
		existingSecret, err := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), datadogSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 1)
		require.Contains(t, existingSecret.Data, "team-a")

		err = datadogIntegration.DeleteIntegration(t.Context(), defaultNamespace, "team-b")
		require.NoError(t, err)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		existingObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: datadogSecretName, Namespace: defaultNamespace},
				Data: map[string][]byte{
					"team-a": []byte(`{"api_key":"key","dd_url":"url"}`),
					"team-b": []byte(`{"api_key":"key2","dd_url":"url2"}`),
				},
			},
		}
		mockK8sClient := fake.NewClientset(existingObjects...)
		datadogIntegration := &DataDogIntegration{
			K8sClient: mockK8sClient,
		}

		// validate secret exists with both integration names before we delete one of them.
		existingSecret, err := mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), datadogSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 2)
		require.Contains(t, existingSecret.Data, "team-a")
		require.Contains(t, existingSecret.Data, "team-b")

		err = datadogIntegration.DeleteIntegration(t.Context(), defaultNamespace, "team-a")
		require.NoError(t, err)

		existingSecret, err = mockK8sClient.CoreV1().Secrets(defaultNamespace).Get(t.Context(), datadogSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, existingSecret.Data)
		require.Len(t, existingSecret.Data, 1)
		require.NotContains(t, existingSecret.Data, "team-a") // team-a was deleted
		require.Contains(t, existingSecret.Data, "team-b")
	})
}
