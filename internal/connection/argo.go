package connection

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

type argoSyncPayload struct {
	Revision  string           `json:"revision"`
	Prune     bool             `json:"prune"`
	DryRun    bool             `json:"dryRun"`
	Strategy  argoSyncStrategy `json:"strategy"`
	Manifests []string         `json:"manifests"`
}

type argoSyncStrategy struct {
	Apply argoSyncApply `json:"apply"`
}

type argoSyncApply struct {
	Force bool `json:"force"`
}

func (oc *OctantConnection) sideloadConnectionApp(
	ctx context.Context,
	logger *zap.Logger,
	namespace, name string,
	connection OctantConnectionData,
) error {
	templateData, err := oc.createTemplateData(ctx, namespace, name, connection)
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
		// TODO: logger
		// logger.Error("unmarshalling cert manager application manifest", zap.Error(err))
		return fmt.Errorf("unmarshaling app manifest: %w", err)
	}

	clientOpts := &apiclient.ClientOptions{
		ServerAddr: argoIntegration.APIUrl,
		AuthToken:  argoIntegration.AccountToken,
		Insecure:   oc.configuration.Env == config.Dev, // ignore certs in localdev
	}
	logger.Debug("pushing app install", zap.String("appName", argoApp.Name))
	if err = oc.argoClient.PushArgoApp(ctx, logger, clientOpts, argoApp); err != nil {
		return fmt.Errorf("pushing argo app: %w", err)
	}

	return oc.doArgoAppSync(ctx, logger, templateData, argoIntegration, name)
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
	name string,
) error {
	manifests, err := renderCollectorDeploymentManifests(templateData, JSONOutputFormat)
	if err != nil {
		return err
	}

	var manifestsSlice []string
	for _, manifest := range manifests {
		manifestsSlice = append(manifestsSlice, string(manifest))
	}

	clientOpts := &apiclient.ClientOptions{
		ServerAddr: argoIntegration.APIUrl,
		AuthToken:  argoIntegration.AccountToken,
		Insecure:   oc.configuration.Env == config.Dev, // ignore certs in localdev
	}
	return oc.argoClient.SyncApplication(ctx, logger, clientOpts, name, manifestsSlice)
}

func (oc *OctantConnection) sideloadValidatorForConnection(
	ctx context.Context,
	logger *zap.Logger,
	integrationName string,
	connectionName string,
	namespace string,
) (string, error) {
	// TODO: This is wonky today; we are pushing a new sync on the same app as the connection, but with just the
	//       validator. This should work because prune = false (argo won't remove those other "orphaned" resources). The
	//       entire sideload behavior has this ephemerality problem... but feels weird to push new manifests on top of
	//       the old ones like this. Clean this up for the git integration.

	argoIntegration, err := oc.getArgoIntegration(ctx, integrationName)
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

	clientOpts := &apiclient.ClientOptions{
		ServerAddr: argoIntegration.APIUrl,
		AuthToken:  argoIntegration.AccountToken,
		Insecure:   oc.configuration.Env == config.Dev, // ignore certs in localdev
	}
	if err = oc.argoClient.SyncApplication(ctx, logger, clientOpts, connectionName, manifestsSlice); err != nil {
		return "", err
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

	clientOpts := &apiclient.ClientOptions{
		ServerAddr: argoIntegration.APIUrl,
		AuthToken:  argoIntegration.AccountToken,
		Insecure:   oc.configuration.Env == config.Dev, // ignore certs in localdev
	}
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

	clientOpts := &apiclient.ClientOptions{
		ServerAddr: argoIntegration.APIUrl,
		AuthToken:  argoIntegration.AccountToken,
		Insecure:   oc.configuration.Env == config.Dev, // ignore certs in localdev
	}
	logger.Debug("deleting telemetry validator app", zap.String("appName", name))
	return oc.argoClient.DeleteArgoApp(ctx, logger, clientOpts, name)
}
