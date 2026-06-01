package rpchandler

import (
	"context"
	"maps"
	"slices"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ArgoCDHandler struct {
	octantv1alphaconnect.UnimplementedArgoCDServiceHandler

	config          *config.Configuration
	argoClient      argocd.APIClient
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData]
}

func NewArgoCDHandler(
	configuration *config.Configuration,
	argoClient argocd.APIClient,
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData],
) *ArgoCDHandler {
	return &ArgoCDHandler{
		config:          configuration,
		argoClient:      argoClient,
		argoIntegration: argoIntegration,
	}
}

func (ah *ArgoCDHandler) TestConnection(
	ctx context.Context,
	req *connect.Request[octantv1alpha.TestConnectionRequest],
) (*connect.Response[octantv1alpha.TestConnectionResponse], error) {
	argoEndpoint := req.Msg.GetArgoEndpoint()
	argoAccountToken := req.Msg.GetArgoAccountToken()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ArgoCDServiceTestConnectionProcedure),
		zap.String("argoEndpoint", argoEndpoint),
	)

	logger.Debug("received request")

	clientOpts := argocd.CreateClientOpts(ah.config.Env, argoEndpoint, argoAccountToken)
	success, err := ah.argoClient.TestConnection(ctx, logger, clientOpts)
	if err != nil {
		logger.Error("testing argocd connection", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse[octantv1alpha.TestConnectionResponse](
		&octantv1alpha.TestConnectionResponse{
			Success: success,
		}), nil
}

func (ah *ArgoCDHandler) SaveArgoConnection(
	ctx context.Context,
	req *connect.Request[octantv1alpha.SaveArgoConnectionRequest],
) (*connect.Response[emptypb.Empty], error) {
	argoEndpoint := req.Msg.GetArgoEndpoint()
	accountToken := req.Msg.GetArgoAccountToken()
	integrationName := req.Msg.GetName()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ArgoCDServiceSaveArgoConnectionProcedure),
		zap.String("argoEndpoint", argoEndpoint),
		zap.String("integrationName", integrationName),
	)

	logger.Debug("received request")

	if err := ah.argoIntegration.SetIntegration(ctx, integrationName,
		integration.ArgoCDIntegrationData{
			APIUrl:       argoEndpoint,
			AccountToken: accountToken,
		}); err != nil {
		logger.Error("setting integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &connect.Response[emptypb.Empty]{}, nil
}

func (ah *ArgoCDHandler) GetArgoIntegrations(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[octantv1alpha.GetArgoIntegrationsResponse], error) {
	logger := zap.L().With(zap.String("operation", octantv1alphaconnect.ArgoCDServiceGetArgoIntegrationsProcedure))

	logger.Debug("received request")

	argoIntegrations, err := ah.argoIntegration.GetIntegrations(ctx)
	if err != nil {
		logger.Error("getting integrations", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse[octantv1alpha.GetArgoIntegrationsResponse](&octantv1alpha.GetArgoIntegrationsResponse{
		Names: slices.Collect(maps.Keys(argoIntegrations)),
	}), nil
}

func (ah *ArgoCDHandler) GetArgoIntegrationByName(
	ctx context.Context,
	req *connect.Request[octantv1alpha.GetArgoIntegrationByNameRequest],
) (*connect.Response[octantv1alpha.GetArgoIntegrationByNameResponse], error) {
	integrationName := req.Msg.GetName()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ArgoCDServiceGetArgoIntegrationsProcedure),
		zap.String("integrationName", integrationName),
	)

	logger.Debug("received request")

	argoData, err := ah.argoIntegration.GetIntegrationByName(ctx, integrationName)
	if err != nil {
		logger.Error("getting integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse[octantv1alpha.GetArgoIntegrationByNameResponse](
		&octantv1alpha.GetArgoIntegrationByNameResponse{
			ArgoEndpoint: argoData.APIUrl,
		},
	), nil
}
