package setting

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
)

var ErrStillUpdating = errors.New("still updating")

type ManagerBuilder interface {
	// Build builds a new setting manager.
	Build(
		ctx context.Context,
		namespace string,
		connectionName string,
		logger *zap.Logger,
	) (Manager, error)

	// Done let the builder know that
	// setting update for the given connection is done.
	// So that the builder will release the lock to allow
	// other threads to perform setting update.
	//
	// PS: ID can be retrieved from Manager.ID().
	Done(
		ctx context.Context,
		connectionName string,
		id string,
	)
}

// SettingManagerBuilder implements ManagerBuilder.
type SettingManagerBuilder struct {
	configuration   *config.Configuration
	connection      connection.Connection[connection.OctantConnectionData]
	datadog         integration.Integration[integration.DataDogIntegrationData]
	argoClient      argocd.APIClient
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData]

	mutex      *sync.Mutex
	inProgress map[string]string
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
		mutex:           new(sync.Mutex),
		inProgress:      map[string]string{},
	}
}

// Build builds a new setting manager.
func (smb *SettingManagerBuilder) Build( //nolint:ireturn
	ctx context.Context,
	namespace string,
	connectionName string,
	logger *zap.Logger,
) (Manager, error) {
	id := uuid.NewString()
	smb.mutex.Lock()
	if _, ok := smb.inProgress[connectionName]; ok {
		smb.mutex.Unlock()
		return nil, ErrStillUpdating
	}
	smb.inProgress[connectionName] = id
	smb.mutex.Unlock()

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
		id:                     id,
		configuration:          smb.configuration,
		connectionService:      smb.connection,
		datadogService:         smb.datadog,
		argoClient:             smb.argoClient,
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

// Done let the builder know that
// setting update for the given connection is done.
// So that the builder will release the lock to allow
// other threads to perform setting update.
//
// PS: ID can be retrieved from Manager.ID().
func (smb *SettingManagerBuilder) Done(
	ctx context.Context,
	connectionName string,
	id string,
) {
	smb.mutex.Lock()
	if val, ok := smb.inProgress[connectionName]; ok && id == val {
		delete(smb.inProgress, connectionName)
	}
	smb.mutex.Unlock()
}
