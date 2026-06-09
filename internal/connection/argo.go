package connection

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
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

	appYAML, err := oc.generator.RenderArgoAppManifest(templateData, YAMLOutputFormat)
	if err != nil {
		return err
	}

	var argoApp argoapp.Application
	if err = yaml.Unmarshal(appYAML, &argoApp); err != nil {
		logger.Error("unmarshalling connection application manifest", zap.Error(err))
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
	manifests, err := oc.generator.RenderCollectorDeploymentManifests(
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
	return oc.argoClient.SyncApplication(
		ctx,
		argocd.Input{
			Logger:     logger,
			ClientOpts: clientOpts,
			AppName:    templateData.ConnectionData.Deployment.IntegrationName,
		},
		manifestsSlice, false)
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

	if waitErr := oc.waitForArgoAppOperation(ctx, logger, connectionName, argoIntegration); waitErr != nil {
		return "", waitErr
	}

	templateData := &ArgoValidatorTemplateData{
		ConnectionName: connectionName,
		Namespace:      namespace,
		ValidatorRunID: getRunID(),
	}

	manifest, err := oc.generator.RenderValidatorManifestForConnection(templateData, YAMLOutputFormat)
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
		argocd.Input{
			Logger:     logger,
			ClientOpts: clientOpts,
			AppName:    connectionName,
		},
		manifestsSlice, false); syncErr != nil {
		return "", syncErr
	}
	return templateData.ValidatorRunID, nil
}

func (oc *OctantConnection) waitForArgoAppOperation(
	ctx context.Context,
	logger *zap.Logger,
	appName string,
	argoIntegration *integration.ArgoCDIntegrationData,
) error {
	clientOpts := argocd.CreateClientOpts(oc.configuration.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	return oc.argoClient.WaitForAppOperation(
		ctx,
		argocd.Input{Logger: logger, ClientOpts: clientOpts, AppName: appName},
		time.Duration(oc.configuration.Install.MdaiInstallPollingIntervalMillis)*time.Millisecond,
		time.Duration(oc.configuration.Install.MdaiInstallTimeout)*time.Second,
	)
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
	return oc.argoClient.DeleteArgoApp(ctx, argocd.Input{
		Logger:     logger,
		ClientOpts: clientOpts,
		AppName:    name,
	})
}

func (oc *OctantConnection) deleteValidatorResource(
	ctx context.Context,
	logger *zap.Logger,
	connectionName string,
	templateData *ArgoConnectionTemplateData,
) error {
	argoIntegration, err := oc.argoIntegration.GetIntegrationByName(
		ctx,
		connectionName,
	)
	if err != nil {
		return err
	}

	// the appTemplates is everything BUT the validator resource, so we'll sync to that and prune out the validator.
	manifests, err := oc.generator.RenderCollectorDeploymentManifests(
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
	return oc.argoClient.SyncApplication(ctx, argocd.Input{
		Logger:     logger,
		ClientOpts: clientOpts,
		AppName:    connectionName,
	}, manifestsSlice, true)
}

func yamlDocsToJSON(yamlBytes []byte) ([]string, error) {
	reader := kyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(yamlBytes)))
	var out []string
	for {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading yaml document: %w", err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		j, err := yaml.YAMLToJSON(doc)
		if err != nil {
			return nil, fmt.Errorf("converting yaml to json: %w", err)
		}
		out = append(out, string(j))
	}
}
