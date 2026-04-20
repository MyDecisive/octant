// Package rpchandler contains handlers that will handle RPC service calls.
package rpchandler

import (
	"context"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
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
	_ context.Context,
	req *connect.Request[octantv1alpha.TestConnectionRequest],
) (*connect.Response[octantv1alpha.TestConnectionResponse], error) {
	argoEndpoint := req.Msg.GetArgoEndpoint()
	_ = req.Msg.GetArgoAccountToken()

	logger := zap.L().With(zap.String("argoEndpoint", argoEndpoint))

	logger.Debug("received test connection request")
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
