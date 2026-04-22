package connection

import (
	"context"
	"fmt"
	"github.com/mydecisive/octant/internal/metrics"
	"github.com/mydecisive/octant/internal/telemetry"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const connectionsConfigmapName = "mdai-octant-connections"

type Status struct {
	ReceivingData     bool                                       `json:"receivingData"`
	SendingData       bool                                       `json:"sendingData"`
	DataIntegrity     bool                                       `json:"dataIntegrity"`
	Details           string                                     `json:"details"`
	ValidationResults map[telemetry.MLT]metrics.ValidationResult `json:"validationResults"`
}

type Connection[T any] interface {
	GetConnectionByName(ctx context.Context, namespace, name string) (*T, error)
	SaveConnection(ctx context.Context, connection T, namespace, connectionName string) error
	DeleteConnection(ctx context.Context, namespace, connectionName string) error
	GetConnectionStatus(ctx context.Context, namespace, connectionName string) (*Status, error)
}

func updateConfigMapWithConnection(ctx context.Context, k8sClient kubernetes.Interface, namespace string, cm *corev1.ConfigMap, connectionName, connectionData string) error {
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[connectionName] = connectionData

	if _, err := k8sClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("error while updating configmap: %w", err)
	}
	return nil
}

func createConnectionConfigMap(ctx context.Context, k8sClient kubernetes.Interface, namespace, configmapName, connectionName, connectionData string) error {
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
