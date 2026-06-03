package connection

import (
	"context"
	"fmt"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const connectionsConfigmapName = "mdai-octant-connections"

type ConnectionCRUDInput struct {
	Logger         *zap.Logger
	Namespace      string
	ConnectionName string
	// Skip does nothing for most connection CRUD,
	// except SaveConnection.
	// If this is set to true for SaveConnection,
	// the operation will only perform ArgoCD app sync and
	// skip saving the connection.
	Skip bool
	// NoDeploy does nothing for most connection CRUD,
	// except SaveConnection.
	// If this is set to true, the operation will skip ArgoCD app sync.
	NoDeploy bool
}

type Connection[T any] interface {
	GetConnectionByName(ctx context.Context, input ConnectionCRUDInput) (*T, error)
	GetConnections(ctx context.Context, input ConnectionCRUDInput) ([]string, error)
	SaveConnection(ctx context.Context, connection T, input ConnectionCRUDInput) error
	DeleteConnection(ctx context.Context, input ConnectionCRUDInput) error
	GetConnectionStatus(
		ctx context.Context,
		input ConnectionCRUDInput,
		validatorRunID string,
	) (*octantv1alpha.GetConnectionStatusResponse, error)
	GetConnectionValidatorRuns(ctx context.Context, input ConnectionCRUDInput) ([]string, error)
	PutConnectionValidatorRun(ctx context.Context, input ConnectionCRUDInput) (string, error)
	DeleteConnectionValidator(ctx context.Context, input ConnectionCRUDInput) error
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
