package setting

import (
	"context"
	"errors"
	"fmt"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var ErrDeploy = errors.New("deploying")

type SettingUpdateResult struct {
	Status octantv1alpha.UpdateResponse_Status
	Err    error
}

type Manager interface {
	// SetDatadogURL provides the manager with the datadog URL to update
	// the datadog integration with.
	// This method will mark the datadog integration as need to be updated.
	SetDatadogURL(url string) Manager
	// SetDatadogAPIKey provides the manager with the datadog API key to update
	// the datadog integration with.
	// This method will mark the datadog integration as need to be updated.
	SetDatadogAPIKey(key string) Manager
	// SetTelemetryTypes provides the manager with the new telemetry types to be enabled.
	// This method will mark the connection as need to be updated.
	SetTelemetryTypes(types []octantv1alpha.MLTType) Manager
	// Apply applies the changes to the corresponding connection, datadog integration, etc
	// using the values provided by various Set* functions.
	Apply(ctx context.Context) error
	// DeployAndWait redeploys the corresponding ArgoCD app to reflect the changes and then
	// it will wait until either the deployment is complete or timeout.
	DeployAndWait(ctx context.Context, out chan SettingUpdateResult)
	// ID returns the ID associated with the current manager.
	// The ID is used to release in progress lock with the manager builder.
	ID() string
}

// SettingManager implements Manager.
// FYI a new instance of setting manager is created via manager builder.
type SettingManager struct {
	configuration     *config.Configuration
	connectionService connection.Connection[connection.OctantConnectionData]
	datadogService    integration.Integration[integration.DataDogIntegrationData]
	argoClient        argocd.APIClient

	logger *zap.Logger

	id             string
	connectionName string
	namespace      string

	connection *connection.OctantConnectionData
	datadog    *integration.DataDogIntegrationData
	argo       *integration.ArgoCDIntegrationData

	shouldUpdateDatadog    bool
	shouldUpdateConnection bool
}

// Ensure SettingManager implements Manager.
var _ Manager = (*SettingManager)(nil)

// ID returns the ID associated with the current manager.
// The ID is used to release in progress lock with the manager builder.
func (sm *SettingManager) ID() string {
	return sm.id
}

// SetDatadogURL provides the manager with the datadog URL to update
// the datadog integration with.
// This method will mark the datadog integration as need to be updated.
func (sm *SettingManager) SetDatadogURL(url string) Manager { //nolint:ireturn
	if url != "" && url != sm.datadog.SiteHost {
		sm.logger = sm.logger.With(zap.String("newDatadogURL", url))
		sm.datadog.SiteHost = url
		sm.shouldUpdateDatadog = true
	}
	return sm
}

// SetDatadogAPIKey provides the manager with the datadog API key to update
// the datadog integration with.
// This method will mark the datadog integration as need to be updated.
func (sm *SettingManager) SetDatadogAPIKey(key string) Manager { //nolint:ireturn
	if key != "" && key != sm.datadog.APIKey {
		sm.logger = sm.logger.With(zap.Bool("haveNewDatadogAPIKey", true))
		sm.datadog.APIKey = key
		sm.shouldUpdateDatadog = true
	}
	return sm
}

// SetTelemetryTypes provides the manager with the new telemetry types to be enabled.
// This method will mark the connection as need to be updated.
func (sm *SettingManager) SetTelemetryTypes(types []octantv1alpha.MLTType) Manager { //nolint:ireturn
	updates := telemetry.ToMLTs(types)
	if len(updates) > 0 {
		removed, added := lo.Difference(sm.connection.TelemetryTypes, updates)
		if len(removed) > 0 || len(added) > 0 {
			sm.logger = sm.logger.With(
				zap.String("removedTelemetries", fmt.Sprint(removed)),
				zap.String("addedTelemetries", fmt.Sprint(added)),
			)
			sm.connection.TelemetryTypes = updates
			sm.shouldUpdateConnection = true
		}
	}
	return sm
}

// Apply applies the changes to the corresponding connection, datadog integration, etc
// using the values provided by various Set* functions.
func (sm *SettingManager) Apply(ctx context.Context) error {
	if sm.shouldUpdateDatadog {
		sm.logger.Debug("updating datadog integration")

		if err := sm.datadogService.SetIntegration(ctx, sm.connectionName, *sm.datadog); err != nil {
			return fmt.Errorf("update datadog integration:%w", err)
		}
	}

	if sm.shouldUpdateConnection {
		sm.logger.Debug("updating connection")
		if err := sm.connectionService.SaveConnection(ctx, *sm.connection, connection.ConnectionCRUDInput{
			Logger:         sm.logger,
			Namespace:      sm.namespace,
			ConnectionName: sm.connectionName,
			NoDeploy:       true,
		}); err != nil {
			return fmt.Errorf("updating connection:%w", err)
		}
	}

	return nil
}

// DeployAndWait redeploys the corresponding ArgoCD app to reflect the changes and then
// it will wait until either the deployment is complete or timeout.
func (sm *SettingManager) DeployAndWait(ctx context.Context, out chan SettingUpdateResult) {
	defer close(out) // Tell caller the operation is complete
	if !sm.shouldUpdateDatadog && !sm.shouldUpdateConnection {
		sm.logger.Debug("nothing to deploy")
		out <- SettingUpdateResult{
			Status: octantv1alpha.UpdateResponse_STATUS_COMPLETED,
		}
		return
	}

	out <- SettingUpdateResult{
		Status: octantv1alpha.UpdateResponse_STATUS_DEPLOY,
	}

	if err := sm.connectionService.SaveConnection(ctx, *sm.connection, connection.ConnectionCRUDInput{
		Logger:         sm.logger,
		Namespace:      sm.namespace,
		ConnectionName: sm.connectionName,
		OnlyDeploy:     true,
	}); err != nil {
		out <- SettingUpdateResult{
			Err: fmt.Errorf("%w:%w", ErrDeploy, err),
		}
		return
	}

	results := make(chan argocd.InstallResult)
	go func() {
		sm.argoClient.AppOperationState(ctx, argocd.Input{
			Logger:     sm.logger,
			ClientOpts: argocd.CreateClientOpts(sm.configuration.Env, sm.argo.APIUrl, sm.argo.AccountToken),
			AppName:    sm.connectionName,
		},
			time.Duration(sm.configuration.Install.MdaiInstallPollingIntervalMillis)*time.Millisecond,
			time.Duration(sm.configuration.Install.MdaiInstallTimeout)*time.Second,
			results)
	}()

	for result := range results {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if result.Err != nil {
			out <- SettingUpdateResult{
				Err: fmt.Errorf("waiting:%w", result.Err),
			}
			return
		}

		out <- SettingUpdateResult{
			Status: sm.toUpdateStatus(result.Status),
		}
	}
}

// toUpdateStatus converts install status to update status.
func (*SettingManager) toUpdateStatus(val octantv1alpha.InstallStatus) octantv1alpha.UpdateResponse_Status {
	switch val {
	case octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED:
		return octantv1alpha.UpdateResponse_STATUS_COMPLETED
	case octantv1alpha.InstallStatus_INSTALL_STATUS_TIMEOUT:
		return octantv1alpha.UpdateResponse_STATUS_TIMEOUT
	default:
		return octantv1alpha.UpdateResponse_STATUS_WAIT
	}
}
