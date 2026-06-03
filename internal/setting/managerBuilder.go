package setting

import (
	"context"
	"errors"
	"fmt"

	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
)

type ManagerBuilder interface {
	// Build builds a new setting manager.
	Build(
		ctx context.Context,
		namespace string,
		connectionName string,
		logger *zap.Logger,
	) (Manager, error)
}

// SettingManagerBuilder implements ManagerBuilder.
type SettingManagerBuilder struct {
	configuration   *config.Configuration
	connection      connection.Connection[connection.OctantConnectionData]
	datadog         integration.Integration[integration.DataDogIntegrationData]
	argoClient      argocd.APIClient
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData]
}

// Ensure SettingManagerBuilder implements ManagerBuilder.
var _ ManagerBuilder = &SettingManagerBuilder{}

// NewSettingManagerBuilder creates a new instance of SettingManagerBuilder.
func NewSettingManagerBuilder(
	configuration *config.Configuration,
	conn connection.Connection[connection.OctantConnectionData],
	datadog integration.Integration[integration.DataDogIntegrationData],
	argoClient argocd.APIClient,
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData],
) *SettingManagerBuilder {
	return &SettingManagerBuilder{
		configuration:   configuration,
		connection:      conn,
		datadog:         datadog,
		argoClient:      argoClient,
		argoIntegration: argoIntegration,
	}
}

// Build builds a new setting manager.
func (smb *SettingManagerBuilder) Build( //nolint:ireturn
	ctx context.Context,
	namespace string,
	connectionName string,
	logger *zap.Logger,
) (Manager, error) {
	con, err := smb.connection.GetConnectionByName(ctx, connection.ConnectionCRUDInput{
		ConnectionName: connectionName,
		Namespace:      namespace,
		Logger:         logger,
	})
	if err != nil {
		return nil, fmt.Errorf("no connection:%w", err)
	}
	if con == nil {
		return nil, errors.New("connection empty")
	}

	dd, err := smb.datadog.GetIntegrationByName(ctx, connectionName)
	if err != nil {
		return nil, fmt.Errorf("no datadog integration:%w", err)
	}
	if dd == nil {
		return nil, errors.New("datadog integration empty")
	}

	argo, err := smb.argoIntegration.GetIntegrationByName(ctx, connectionName)
	if err != nil {
		return nil, fmt.Errorf("no argocd integration:%w", err)
	}
	if argo == nil {
		return nil, errors.New("argo integration empty")
	}

	return &SettingManager{
		configuration:          smb.configuration,
		connectionService:      smb.connection,
		datadogService:         smb.datadog,
		argoClient:             smb.argoClient,
		argoService:            smb.argoIntegration,
		logger:                 logger,
		connectionName:         connectionName,
		namespace:              namespace,
		connection:             con,
		datadog:                dd,
		argo:                   argo,
		shouldUpdateDatadog:    false,
		shouldUpdateConnection: false,
	}, nil
}
