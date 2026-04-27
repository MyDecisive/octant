package integration

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Integration[T any] interface {
	GetIntegrations(ctx context.Context, namespace string) (map[string]T, error)
	GetIntegrationByName(ctx context.Context, namespace, name string) (*T, error)
	SetIntegration(ctx context.Context, namespace, integrationName string, integrationData T) error
	DeleteIntegration(ctx context.Context, namespace, integrationName string) error
}

func updateSecretWithIntegration(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	namespace string,
	secret *corev1.Secret,
	jsonData []byte,
) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[secret.Name] = jsonData

	_, err := k8sClient.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func createIntegrationSecret(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	namespace, secretName string,
	jsonData []byte,
) error {
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretName: jsonData,
		},
		Type: corev1.SecretTypeOpaque,
	}

	if _, err := k8sClient.CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create secret %s: %w", secretName, err)
	}
	return nil
}
