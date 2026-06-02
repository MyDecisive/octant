package connection

import (
	"context"
	"encoding/json"
	"fmt"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/mdai-data-core/kube"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/metrics"
	"github.com/mydecisive/octant/internal/telemetry"
	"go.uber.org/zap"
)

type Validator interface {
	GetConnectionValidatorRuns(ctx context.Context, input ConnectionCRUDInput) ([]string, error)
	PutConnectionValidatorRun(ctx context.Context, input ConnectionCRUDInput, deployType DeploymentType) (string, error)
	DeleteConnectionValidator(ctx context.Context, input ConnectionCRUDInput) error
	GetConnectionStatus(
		ctx context.Context,
		input ConnectionCRUDInput,
		validatorRunID string,
		telemetryTypes []telemetry.MLT,
	) (*octantv1alpha.GetConnectionStatusResponse, error)
}

type OctantConnectionValidator struct {
	configuration      config.Configuration
	connectionMetrics  metrics.ConnectionStatus
	argocdIntegration  integration.Integration[integration.ArgoCDIntegrationData]
	datadogIntegration integration.Integration[integration.DataDogIntegrationData]
	manifestGenerator  ManifestGenerator
	argoClient         argocd.APIClient
	cmStore            kube.ConfigMapStore
}

func (oc *OctantConnectionValidator) GetConnectionValidatorRuns(
	ctx context.Context,
	input ConnectionCRUDInput,
) ([]string, error) {
	return oc.connectionMetrics.GetConnectionValidatorRuns(ctx, input.Namespace, input.ConnectionName)
}

func (oc *OctantConnectionValidator) GetConnectionStatus(
	ctx context.Context,
	input ConnectionCRUDInput,
	validatorRunID string,
	telemetryTypes []telemetry.MLT,
) (
	*octantv1alpha.GetConnectionStatusResponse,
	error,
) {
	return oc.connectionMetrics.GetConnectionStatus(
		ctx,
		input.Namespace,
		input.ConnectionName,
		telemetryTypes,
		validatorRunID,
	)
}

func (oc *OctantConnectionValidator) PutConnectionValidatorRun(ctx context.Context, input ConnectionCRUDInput, deployType DeploymentType) (string, error) {
	if deployType == ArgoSideloadDeploymentType {
		return oc.sideloadValidatorForConnection(ctx, input.Logger, input.ConnectionName, input.Namespace)
	}

	input.Logger.Warn("no-op validator run for non-sideload deployment")
	return "", nil
}

func (oc *OctantConnectionValidator) DeleteConnectionValidator(ctx context.Context, input ConnectionCRUDInput) error {
	cm, getCMErr := oc.cmStore.GetConfigmapByNameAndNamespace(
		connectionsConfigmapName,
		oc.configuration.CurrentNamespace,
	)
	if getCMErr != nil {
		input.Logger.Warn("fetching connection configmap", zap.Error(getCMErr))
		return fmt.Errorf("failed to fetch configmap %s: %w", connectionsConfigmapName, getCMErr)
	}

	if _, exists := cm.Data[input.ConnectionName]; !exists {
		input.Logger.Warn("connection not found in configmap", zap.String("connectionName", input.ConnectionName))
		return fmt.Errorf("connection not found in configmap %s", input.ConnectionName)
	}

	var connection OctantConnectionData
	if err := json.Unmarshal([]byte(cm.Data[input.ConnectionName]), &connection); err != nil {
		return fmt.Errorf("failed to unmarshal connection data: %w", err)
	}

	// TODO: This should be refactored to a more robust deployment-based task system
	if connection.Deployment != nil && connection.Deployment.Type == ArgoSideloadDeploymentType {
		templateData, err := createTemplateData(ctx, input.ConnectionName, connection, oc.datadogIntegration, oc.configuration)
		if err != nil {
			return err
		}
		return oc.deleteValidatorResource(ctx, input.Logger, input.ConnectionName, templateData)
	}

	return nil
}

func (oc *OctantConnectionValidator) sideloadValidatorForConnection(
	ctx context.Context,
	logger *zap.Logger,
	connectionName string,
	namespace string,
) (string, error) {
	// TODO: This is wonky today; we are pushing a new sync on the same app as the connection, but with just the
	//       validator. This should work because prune = false (argo won't remove those other "orphaned" resources). The
	//       entire sideload behavior has this ephemerality problem... but feels weird to push new manifests on top of
	//       the old ones like this. Clean this up for the git integration.
	argoIntegration, err := oc.argocdIntegration.GetIntegrationByName(ctx, connectionName)
	if err != nil {
		return "", fmt.Errorf("getting ArgoCD integration: %w", err)
	}

	templateData := &ArgoValidatorTemplateData{
		ConnectionName: connectionName,
		Namespace:      namespace,
		ValidatorRunID: getRunID(),
	}

	manifest, err := oc.manifestGenerator.RenderValidatorManifestForConnection(templateData, YAMLOutputFormat)
	if err != nil {
		return "", err
	}

	manifestsSlice, err := yamlDocsToJSON(manifest)
	if err != nil {
		return "", fmt.Errorf("preparing validator manifest for argo sync: %w", err)
	}

	clientOpts := argocd.CreateClientOpts(oc.configuration.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	if syncErr := oc.argoClient.SyncApplication(
		ctx,
		logger,
		clientOpts,
		connectionName,
		manifestsSlice, false); syncErr != nil {
		return "", syncErr
	}
	return templateData.ValidatorRunID, nil
}

func (oc *OctantConnectionValidator) deleteValidatorResource(
	ctx context.Context,
	logger *zap.Logger,
	connectionName string,
	templateData *ArgoConnectionTemplateData,
) error {
	argoIntegration, err := oc.argocdIntegration.GetIntegrationByName(
		ctx,
		connectionName,
	)
	if err != nil {
		return err
	}

	// the appTemplates is everything BUT the validator resource, so we'll sync to that and prune out the validator.
	manifests, err := oc.manifestGenerator.RenderCollectorDeploymentManifests(
		templateData,
		getDefaultAppTemplates(),
		YAMLOutputFormat,
	)
	if err != nil {
		return err
	}

	var manifestsSlice []string
	for _, manifest := range manifests {
		docs, convertErr := yamlDocsToJSON(manifest)
		if convertErr != nil {
			return fmt.Errorf("preparing manifests for argo sync: %w", convertErr)
		}
		manifestsSlice = append(manifestsSlice, docs...)
	}

	clientOpts := argocd.CreateClientOpts(oc.configuration.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	logger.Debug("deleting telemetry validator resource")
	// WITH prune so the validator resource gets removed.
	return oc.argoClient.SyncApplication(ctx, logger, clientOpts, connectionName, manifestsSlice, true)
}
