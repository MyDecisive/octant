package integration

import (
	"context"

	"github.com/mydecisive/mdai-data-core/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Integration[T any] interface {
	GetIntegrations(ctx context.Context) (map[string]T, error)
	GetIntegrationByName(ctx context.Context, name string) (*T, error)
	SetIntegration(ctx context.Context, integrationName string, integrationData T) error
	DeleteIntegration(ctx context.Context, integrationName string) error
}

func updateSecretWithIntegration(
	ctx context.Context,
	secretStore kube.SecretStore,
	namespace, integrationName string,
	secret *corev1.Secret,
	jsonData []byte,
) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[integrationName] = jsonData

	return secretStore.UpdateSecret(ctx, namespace, secret)
}

func createIntegrationSecret(
	ctx context.Context,
	secretStore kube.SecretStore,
	namespace, integrationName, secretName, secretTypeLabel string,
	jsonData []byte,
) error {
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				kube.SecretTypeLabel: secretTypeLabel,
			},
		},
		Data: map[string][]byte{
			integrationName: jsonData,
		},
		Type: corev1.SecretTypeOpaque,
	}
	return secretStore.CreateSecret(ctx, namespace, newSecret)
}
