package connection

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

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
	templateData, err := createTemplateData(ctx, name, connection, oc.datadogIntegration, *oc.configuration)
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
		logger,
		clientOpts,
		templateData.ConnectionData.Deployment.IntegrationName,
		manifestsSlice, false)
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
