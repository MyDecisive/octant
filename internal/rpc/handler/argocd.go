// Package rpchandler contains handlers that will handle RPC service calls.
package rpchandler

import (
	"context"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/mydecisive/octant/internal/config"
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

type ArgoCDHandler struct {
	octantv1alphaconnect.UnimplementedArgoCDServiceHandler

	config *config.Configuration
}

func NewArgoCDHandler(config *config.Configuration) *ArgoCDHandler {
	return &ArgoCDHandler{
		config: config,
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

	argoClient, err := apiclient.NewClient(&apiclient.ClientOptions{
		ServerAddr: argoEndpoint,
		AuthToken:  argoAccountToken,
		Insecure:   ah.config.Env == config.Dev, // ignore certs in localdev
	})
	if err != nil {
		logger.Error("creating argo api client", zap.Error(err))
		return &connect.Response[octantv1alpha.TestConnectionResponse]{
			Msg: &octantv1alpha.TestConnectionResponse{
				Success: false,
			},
		}, nil
	}

	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		logger.Error("creating argo application client", zap.Error(err))
		return &connect.Response[octantv1alpha.TestConnectionResponse]{
			Msg: &octantv1alpha.TestConnectionResponse{
				Success: false,
			},
		}, nil
	}
	defer closer.Close()

	// to validate the account token, we'll query for a list of applications, which requires a valid account token.
	_, err = applicationClient.List(ctx, &application.ApplicationQuery{
		Name: lo.ToPtr("mdai"),
	})
	if err != nil {
		logger.Error("getting argo application list", zap.Error(err))
		return &connect.Response[octantv1alpha.TestConnectionResponse]{
			Msg: &octantv1alpha.TestConnectionResponse{
				Success: false,
			},
		}, nil
	}

	return &connect.Response[octantv1alpha.TestConnectionResponse]{
		Msg: &octantv1alpha.TestConnectionResponse{
			Success: true,
		},
	}, nil
}

func (ah *ArgoCDHandler) SaveArgoConnection(
	_ context.Context,
	req *connect.Request[octantv1alpha.SaveArgoConnectionRequest],
) (*connect.Response[emptypb.Empty], error) {
	argoEndpoint := req.Msg.GetArgoEndpoint()
	_ = req.Msg.GetArgoAccountToken()

	logger := zap.L().With(zap.String("argoEndpoint", argoEndpoint))

	logger.Debug("received save connection request")
	return &connect.Response[emptypb.Empty]{}, nil
}
