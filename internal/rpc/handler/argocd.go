// Package rpchandler contains handlers that will handle RPC service calls.
package rpchandler

import (
	"connectrpc.com/connect"
	"context"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
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
	config *config.Configuration,
	argoClient argocd.APIClient,
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData],
) *ArgoCDHandler {
	return &ArgoCDHandler{
		config:          config,
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

	logger := zap.L().With(zap.String("argoEndpoint", argoEndpoint))

	logger.Debug("received test connection request")

	clientOpts := &apiclient.ClientOptions{
		ServerAddr: argoEndpoint,
		AuthToken:  argoAccountToken,
		Insecure:   ah.config.Env == config.Dev, // ignore certs in localdev
	}
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

	logger := zap.L().With(zap.String("argoEndpoint", argoEndpoint))

	logger.Debug("received save connection request")

	err := ah.argoIntegration.SetIntegration(ctx, ah.config.InstallNamespace, "mdai", integration.ArgoCDIntegrationData{
		APIUrl:       argoEndpoint,
		AccountToken: accountToken,
	})
	if err != nil {
		logger.Error("setting integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &connect.Response[emptypb.Empty]{}, nil
}
