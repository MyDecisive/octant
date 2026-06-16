package connection

import (
	"context"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
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
	// OnlyDeploy does nothing for most connection CRUD,
	// except SaveConnection.
	// If this is set to true for SaveConnection,
	// the operation will only perform ArgoCD app sync and
	// skip saving the connection.
	OnlyDeploy bool
	// NoDeploy does nothing for most connection CRUD,
	// except SaveConnection.
	// If this is set to true, the operation will skip ArgoCD app sync.
	NoDeploy bool
}

type CompressionInput struct {
	Namespace      string
	Connection     string
	MdaiVersion    string
	Telemetries    []octantv1alpha.MLTType
	Format         octantv1alpha.ManifestOutFormat
	DeploymentType octantv1alpha.DeploymentType
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
