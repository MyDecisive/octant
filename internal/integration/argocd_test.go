package integration

import (
	"encoding/json"
	v1 "github.com/mydecisive/octant/api/v1"
	"testing"

	"github.com/mydecisive/mdai-data-core/kube"
	kubemock "github.com/mydecisive/mdai-data-core/mock/kube"
	"github.com/mydecisive/octant/internal/config"
	installlogmock "github.com/mydecisive/octant/internal/mock/installlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		installLogStore := installlogmock.NewMockInstallLogStore(t)
		argocdIntegration := &ArgoCDIntegration{
			secretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			installLogStore: installLogStore,
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
		installLogStore := installlogmock.NewMockInstallLogStore(t)
		argocdIntegration := &ArgoCDIntegration{
			secretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			installLogStore: installLogStore,
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
		installLogStore := installlogmock.NewMockInstallLogStore(t)
		argocdIntegration := &ArgoCDIntegration{
			secretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			installLogStore: installLogStore,
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

		notFoundError := k8serrors.NewNotFound(schema.GroupResource{}, argocdSecretName)
		secretStore := kubemock.NewMockSecretStore(t)
		secretStore.EXPECT().
			GetSecretByNameAndNamespace(argocdSecretName, defaultNamespace).
			Return(nil, notFoundError).
			Once()
		secretStore.EXPECT().
			CreateSecret(mock.Anything, defaultNamespace, mock.MatchedBy(func(secret *corev1.Secret) bool {
				require.Contains(t, secret.Data, "team-a")
				return secret.Name == argocdSecretName
			})).
			Return(nil).
			Once()
		installLogStore := installlogmock.NewMockInstallLogStore(t)
		installLogStore.EXPECT().AddInstallLogEvent(
			mock.Anything,
			mock.MatchedBy(func(event *v1.OctantInstallEvent) bool {
				return event.Action == v1.CreateDeployIntegrationOctantInstallEventAction &&
					event.Namespace == defaultNamespace &&
					event.Ref == "team-a" &&
					event.Result == v1.SuccessOctantInstallEventResult
			}),
		).Return(nil).Once()
		argocdIntegration := &ArgoCDIntegration{
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			secretStore:     secretStore,
			installLogStore: installLogStore,
		}

		err := argocdIntegration.SetIntegration(t.Context(), "team-a", newIntegration)
		require.NoError(t, err)
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
		secretStore.EXPECT().
			UpdateSecret(mock.Anything, defaultNamespace, mock.MatchedBy(func(secret *corev1.Secret) bool {
				require.Contains(t, secret.Data, "team-a")
				require.Contains(t, secret.Data, "team-b")
				return secret.Name == argocdSecretName
			})).
			Return(nil).
			Once()

		installLogStore := installlogmock.NewMockInstallLogStore(t)
		installLogStore.EXPECT().AddInstallLogEvent(
			mock.Anything,
			mock.MatchedBy(func(event *v1.OctantInstallEvent) bool {
				return event.Action == v1.CreateDeployIntegrationOctantInstallEventAction &&
					event.Namespace == defaultNamespace &&
					event.Ref == "team-b" &&
					event.Result == v1.SuccessOctantInstallEventResult
			}),
		).Return(nil).Once()
		datadogIntegration := &ArgoCDIntegration{
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			secretStore:     secretStore,
			installLogStore: installLogStore,
		}

		require.NoError(t, datadogIntegration.SetIntegration(t.Context(), "team-b", newIntegration))
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
		installLogStore := installlogmock.NewMockInstallLogStore(t)
		datadogIntegration := &ArgoCDIntegration{
			secretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			installLogStore: installLogStore,
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
		installLogStore := installlogmock.NewMockInstallLogStore(t)
		argocdIntegration := &ArgoCDIntegration{
			secretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			installLogStore: installLogStore,
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
		secretStore.EXPECT().
			UpdateSecret(mock.Anything, defaultNamespace, mock.MatchedBy(func(secret *corev1.Secret) bool {
				return secret.Name == argocdSecretName &&
					secret.Data["team-a"] == nil && // this one was deleted.
					secret.Data["team-b"] != nil
			})).
			Return(nil).
			Once()
		installLogStore := installlogmock.NewMockInstallLogStore(t)

		argocdIntegration := &ArgoCDIntegration{
			secretStore: secretStore,
			configuration: &config.Configuration{
				CurrentNamespace: defaultNamespace,
			},
			installLogStore: installLogStore,
		}

		require.NoError(t, argocdIntegration.DeleteIntegration(t.Context(), "team-a"))
	})
}
