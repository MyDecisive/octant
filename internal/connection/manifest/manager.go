package manifest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type ManagerInput struct {
	Logger                    *zap.Logger
	DeploymentIntegrationName string
	ConnectionName            string
}

// Manager manages loading/unloading apps/manifests from the system.
type Manager interface {
	// Unload removes all given apps from the system.
	Unload(
		ctx context.Context,
		input ManagerInput,
		apps []manifestdata.App,
	) error
	// LoadCertManager install cert manager into the system.
	LoadCertManager(
		ctx context.Context,
		input ManagerInput,
		data manifestdata.AppTemplateData,
	) error
	// LoadMDAI install MDAI hub into the system.
	LoadMDAI(
		ctx context.Context,
		input ManagerInput,
		data manifestdata.AppTemplateData,
	) error
	// LoadConnection installs connection app and connection manifests into the system.
	LoadConnection(
		ctx context.Context,
		logger *zap.Logger,
		input manifestdata.ConnectionInput,
	) error
	// LoadValidator installs validator app and validator manifests into the system.
	LoadValidator(
		ctx context.Context,
		logger *zap.Logger,
		input manifestdata.ValidatorInput,
	) error
}

// Ensure ArgoCDManager implements Manager.
var _ Manager = (*ArgoCDManager)(nil)

type ArgoCDManager struct {
	config     *config.Configuration
	argo       integration.Integration[integration.ArgoCDIntegrationData]
	argoClient argocd.APIClient
	generator  Generator
}

// NewArgoCDManager returns a new instance of ArgoCDManager.
func NewArgoCDManager(
	conf *config.Configuration,
	argo integration.Integration[integration.ArgoCDIntegrationData],
	argoClient argocd.APIClient,
	generator Generator,
) *ArgoCDManager {
	return &ArgoCDManager{
		config:     conf,
		argo:       argo,
		argoClient: argoClient,
		generator:  generator,
	}
}

func (am *ArgoCDManager) Unload(
	ctx context.Context,
	input ManagerInput,
	apps []manifestdata.App,
) error {
	argoClientOpt, err := am.argoClientOpt(ctx, input.DeploymentIntegrationName)
	if err != nil {
		return err
	}

	for _, app := range apps {
		if err := am.argoClient.DeleteArgoApp(ctx, argocd.Input{
			Logger:     input.Logger,
			ClientOpts: argoClientOpt,
			AppName:    am.getAppName(app, input.ConnectionName),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (am *ArgoCDManager) LoadCertManager(
	ctx context.Context,
	input ManagerInput,
	data manifestdata.AppTemplateData,
) error {
	argoClientOpt, err := am.argoClientOpt(ctx, input.DeploymentIntegrationName)
	if err != nil {
		return err
	}
	return am.loadApp(ctx, input.Logger, manifestdata.CERT, data, argoClientOpt)
}

func (am *ArgoCDManager) LoadMDAI(
	ctx context.Context,
	input ManagerInput,
	data manifestdata.AppTemplateData,
) error {
	argoClientOpt, err := am.argoClientOpt(ctx, input.DeploymentIntegrationName)
	if err != nil {
		return err
	}
	return am.loadApp(ctx, input.Logger, manifestdata.MDAI, data, argoClientOpt)
}

func (am *ArgoCDManager) LoadConnection(
	ctx context.Context,
	logger *zap.Logger,
	input manifestdata.ConnectionInput,
) error {
	raw, err := am.generator.Connections(ctx, input, manifestdata.JSON)
	if err != nil {
		return err
	}
	connections := lo.MapToSlice(raw, func(_ string, content []byte) string {
		return string(content)
	})

	return am.load(
		ctx,
		logger,
		input.DeploymentIntegrationName,
		manifestdata.CONNECTION,
		manifestdata.AppTemplateData{
			Name:      input.ConnectionName,
			Namespace: input.Namespace,
		},
		connections)
}

func (am *ArgoCDManager) LoadValidator(
	ctx context.Context,
	logger *zap.Logger,
	input manifestdata.ValidatorInput,
) error {
	raw, err := am.generator.Validators(input, manifestdata.JSON)
	if err != nil {
		return err
	}
	validators := lo.MapToSlice(raw, func(_ string, content []byte) string {
		return string(content)
	})

	return am.load(
		ctx,
		logger,
		input.DeploymentIntegrationName,
		manifestdata.VALIDATOR,
		manifestdata.AppTemplateData{
			Name:      input.ConnectionName,
			Namespace: input.Namespace,
		},
		validators)
}

func (am *ArgoCDManager) load(
	ctx context.Context,
	logger *zap.Logger,
	deploymentIntegrationName string,
	appType manifestdata.App,
	appData manifestdata.AppTemplateData,
	manifests []string,
) error {
	argoClientOpt, err := am.argoClientOpt(ctx, deploymentIntegrationName)
	if err != nil {
		return err
	}

	if errLoad := am.loadApp(ctx, logger, appType, appData, argoClientOpt); errLoad != nil {
		return errLoad
	}

	if err = am.argoClient.SyncApplication(
		ctx,
		argocd.Input{
			Logger:     logger,
			ClientOpts: argoClientOpt,
			AppName:    am.getAppName(appType, appData.Name),
		},
		manifests, false); err != nil {
		return fmt.Errorf("%w: %w", ErrPushManifests, err)
	}
	return nil
}

func (am *ArgoCDManager) loadApp(
	ctx context.Context,
	logger *zap.Logger,
	appType manifestdata.App,
	appData manifestdata.AppTemplateData,
	argoClientOpt *apiclient.ClientOptions,
) error {
	app, err := am.getAppAsArgoCDApp(appType, appData)
	if err != nil {
		return err
	}

	if err = am.argoClient.PushArgoApp(ctx, logger, argoClientOpt, *app); err != nil {
		return fmt.Errorf("%w: %w", ErrPushApp, err)
	}
	return nil
}

func (am *ArgoCDManager) argoClientOpt(ctx context.Context, name string) (*apiclient.ClientOptions, error) {
	argo, err := am.argo.GetIntegrationByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("%w:%w", manifestdata.ErrIntegration, err)
	}
	return argocd.CreateClientOpts(am.config.Env, argo.APIUrl, argo.AccountToken), nil
}

func (*ArgoCDManager) getAppName(app manifestdata.App, connectionName string) string {
	switch app {
	case manifestdata.VALIDATOR:
		return fmt.Sprintf(validatorAppNameFormatter, connectionName)
	case manifestdata.MDAI:
		return "mdai"
	case manifestdata.CERT:
		return "cert-manager"
	default:
		return connectionName
	}
}

func (am *ArgoCDManager) getAppAsArgoCDApp(
	app manifestdata.App,
	data manifestdata.AppTemplateData,
) (*argoapp.Application, error) {
	raw, err := am.generator.App(app, data, manifestdata.JSON)
	if err != nil {
		return nil, err
	}

	var argoApp *argoapp.Application
	if err = json.Unmarshal(raw, &argoApp); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParseTemplate, err)
	}

	return argoApp, nil
}
