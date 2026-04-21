// Package rpchandler contains handlers that will handle RPC service calls.
package rpchandler

import (
	"context"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"google.golang.org/protobuf/types/known/emptypb"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

type ArgoCDHandler struct {
	octantv1alphaconnect.UnimplementedArgoCDServiceHandler
}

func NewArgoCDHandler() *ArgoCDHandler {
	return &ArgoCDHandler{}
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
	})
	if err != nil {
		logger.Error("creating argo api client", zap.Error(err))
		return &connect.Response[octantv1alpha.TestConnectionResponse]{
			Msg: &octantv1alpha.TestConnectionResponse{
				Success: false,
			},
		}, nil
	}

	closer, versionClient, err := argoClient.NewVersionClient()
	if err != nil {
		logger.Error("creating argo version client", zap.Error(err))
		return &connect.Response[octantv1alpha.TestConnectionResponse]{
			Msg: &octantv1alpha.TestConnectionResponse{
				Success: false,
			},
		}, nil
	}
	defer closer.Close()

	// to validate the argocd account token, we'll just query the server version, which will validate our account token.
	_, err = versionClient.Version(ctx, nil)
	if err != nil {
		logger.Error("getting argo version", zap.Error(err))
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
