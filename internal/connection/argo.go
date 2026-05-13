package connection

import (
	"context"
	"fmt"

	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

func (oc *OctantConnection) sideloadConnectionApp(
	ctx context.Context,
	logger *zap.Logger,
	name string,
	connection OctantConnectionData,
) error {
	templateData, err := oc.createTemplateData(ctx, name, connection)
	if err != nil {
		return err
	}

	argoIntegration, err := oc.getArgoIntegration(ctx, connection.Deployment.IntegrationName)
	if err != nil {
		return err
	}

	appYAML, err := renderArgoAppManifest(templateData, YAMLOutputFormat)
	if err != nil {
		return err
	}

	var argoApp argoapp.Application
	if err = yaml.Unmarshal(appYAML, &argoApp); err != nil {
		logger.Error("unmarshalling cert manager application manifest", zap.Error(err))
		return fmt.Errorf("unmarshaling app manifest: %w", err)
	}

	clientOpts := argocd.CreateClientOpts(oc.configuration.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	logger.Debug("pushing app install", zap.String("appName", argoApp.Name))
	if err = oc.argoClient.PushArgoApp(ctx, logger, clientOpts, argoApp); err != nil {
		logger.Error("pushing argo app", zap.Error(err))
		return fmt.Errorf("pushing argo app: %w", err)
	}

	return oc.doArgoAppSync(ctx, logger, templateData, argoIntegration)
}

func (oc *OctantConnection) getArgoIntegration(
	ctx context.Context,
	integrationName string,
) (*integration.ArgoCDIntegrationData, error) {
	argoIntegration, err := oc.argoIntegration.GetIntegrationByName(
		ctx,
		integrationName,
	)
	if err != nil {
		return nil, fmt.Errorf("getting ArgoCD integration: %w", err)
	}
	if argoIntegration == nil {
		return nil, fmt.Errorf("no ArgoCD integration found with name %s", integrationName)
	}
	return argoIntegration, nil
}

func (oc *OctantConnection) doArgoAppSync(
	ctx context.Context,
	logger *zap.Logger,
	templateData *ArgoConnectionTemplateData,
	argoIntegration *integration.ArgoCDIntegrationData,
) error {
	manifests, err := renderCollectorDeploymentManifests(templateData, YAMLOutputFormat)
	if err != nil {
		return err
	}

	var manifestsSlice []string
	for _, manifest := range manifests {
		manifestsSlice = append(manifestsSlice, string(manifest))
	}

	clientOpts := argocd.CreateClientOpts(oc.configuration.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	// TODO: not sure if templateData.AppName or connection name here...
	return oc.argoClient.SyncApplication(ctx, logger, clientOpts, templateData.AppName, manifestsSlice)
}

func (oc *OctantConnection) sideloadValidatorForConnection(
	ctx context.Context,
	logger *zap.Logger,
	connectionName string,
	namespace string,
) (string, error) {
	// TODO: This is wonky today; we are pushing a new sync on the same app as the connection, but with just the
	//       validator. This should work because prune = false (argo won't remove those other "orphaned" resources). The
	//       entire sideload behavior has this ephemerality problem... but feels weird to push new manifests on top of
	//       the old ones like this. Clean this up for the git integration.

	argoIntegration, err := oc.getArgoIntegration(ctx, connectionName)
	if err != nil {
		return "", err
	}

	templateData := &ArgoValidatorTemplateData{
		ConnectionName: connectionName,
		Namespace:      namespace,
		ValidatorRunID: getRunID(),
	}

	manifest, err := renderValidatorManifestForConnection(templateData, YAMLOutputFormat)
	if err != nil {
		return "", err
	}

	manifestsSlice := []string{
		string(manifest),
	}

	clientOpts := argocd.CreateClientOpts(oc.configuration.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	if syncErr := oc.argoClient.SyncApplication(ctx, logger, clientOpts, connectionName, manifestsSlice); syncErr != nil {
		return "", syncErr
	}
	return templateData.ValidatorRunID, nil
}

func (oc *OctantConnection) deleteArgoApp(
	ctx context.Context,
	logger *zap.Logger,
	name string,
	connection OctantConnectionData,
) error {
	argoIntegration, err := oc.argoIntegration.GetIntegrationByName(
		ctx,
		connection.Deployment.IntegrationName,
	)
	if err != nil {
		return err
	}

	clientOpts := argocd.CreateClientOpts(oc.configuration.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	logger.Debug("deleting argo app", zap.String("appName", name))
	return oc.argoClient.DeleteArgoApp(ctx, logger, clientOpts, name)
}

func (oc *OctantConnection) deleteValidatorResource(
	ctx context.Context,
	logger *zap.Logger,
	name string,
	connection OctantConnectionData,
) error {
	argoIntegration, err := oc.argoIntegration.GetIntegrationByName(
		ctx,
		connection.Deployment.IntegrationName,
	)
	if err != nil {
		return err
	}

	clientOpts := argocd.CreateClientOpts(oc.configuration.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	logger.Debug("deleting telemetry validator app", zap.String("appName", name))
	return oc.argoClient.DeleteArgoApp(ctx, logger, clientOpts, name)
}
