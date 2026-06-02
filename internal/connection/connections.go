package connection

import (
	"context"

	"github.com/mydecisive/mdai-data-core/kube"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const connectionsConfigmapName = "mdai-octant-connections"

type ConnectionCRUDInput struct {
	Logger         *zap.Logger
	Namespace      string
	ConnectionName string
}

type Connection interface {
	GetConnectionByName(ctx context.Context, input ConnectionCRUDInput) (*OctantConnectionData, error)
	GetConnections(ctx context.Context, input ConnectionCRUDInput) ([]string, error)
	SaveConnection(ctx context.Context, connection OctantConnectionData, input ConnectionCRUDInput) error
	DeleteConnection(ctx context.Context, input ConnectionCRUDInput) error
}

func updateConfigMapWithConnection(
	ctx context.Context,
	cmStore kube.ConfigMapStore,
	namespace string,
	cm *corev1.ConfigMap,
	connectionName, connectionData string,
) error {
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[connectionName] = connectionData

	return cmStore.UpdateConfigMap(ctx, namespace, cm)
}

func createConnectionConfigMap(
	ctx context.Context,
	cmStore kube.ConfigMapStore,
	namespace, configmapName, connectionName, connectionData string,
) error {
	newCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
			Labels: map[string]string{
				kube.ConfigMapTypeLabel: kube.OctantConnectionsConfigMapType,
			},
		},
		Data: map[string]string{
			connectionName: connectionData,
		},
	}

	return cmStore.CreateConfigMap(ctx, namespace, newCM)
}
