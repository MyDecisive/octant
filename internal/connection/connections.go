package connection

import (
	"context"
	"fmt"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const connectionsConfigmapName = "mdai-octant-connections"

type Connection[T any] interface {
	GetConnectionByName(ctx context.Context, namespace, name string) (*T, error)
	GetConnections(ctx context.Context, namespace string) ([]string, error)
	SaveConnection(ctx context.Context, connection T, namespace, connectionName string) error
	DeleteConnection(ctx context.Context, namespace, connectionName string) error
	GetConnectionStatus(
		ctx context.Context,
		namespace string,
		connectionName string,
		validatorRunID string,
	) (
		*octantv1alpha.GetConnectionStatusResponse,
		error,
	)
	GetConnectionValidatorRuns(ctx context.Context, namespace, connectionName string) ([]string, error)
	PutConnectionValidatorRun(ctx context.Context, namespace, connectionName string) (string, error)
	DeleteConnectionValidator(ctx context.Context, namespace, connectionName string) error
}

func updateConfigMapWithConnection(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	namespace string,
	cm *corev1.ConfigMap,
	connectionName, connectionData string,
) error {
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[connectionName] = connectionData

	if _, err := k8sClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("error while updating configmap: %w", err)
	}
	return nil
}

func createConnectionConfigMap(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	namespace, configmapName, connectionName, connectionData string,
) error {
	newCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			connectionName: connectionData,
		},
	}

	if _, err := k8sClient.CoreV1().ConfigMaps(namespace).Create(ctx, newCM, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create configmap %s: %w", configmapName, err)
	}
	return nil
}
