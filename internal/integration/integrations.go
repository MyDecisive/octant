package integration

import (
	"context"
)

const integrationSecretName = "mdai-gateway-integration"

type Integration[T any] interface {
	GetIntegrations(ctx context.Context, namespace string) (map[string]T, error)
	SetIntegration(ctx context.Context, namespace, integrationName string, integrationData T) error
	DeleteIntegration(ctx context.Context, namespace, integrationName string) error
}
