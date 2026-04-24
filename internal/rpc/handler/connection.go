// Package rpchandler contains handlers that will handle RPC service calls.
package rpchandler

import (
	"connectrpc.com/connect"
	"context"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
)

type ConnectionHandler struct {
	octantv1alphaconnect.UnimplementedConnectionServiceHandler

	config           *config.Configuration
	octantConnection connection.Connection[connection.OctantConnectionData]
}

func NewConnectionHandler(
	config *config.Configuration,
	octantConnection connection.Connection[connection.OctantConnectionData],
) *ConnectionHandler {
	return &ConnectionHandler{
		config:           config,
		octantConnection: octantConnection,
	}
}

func (ch ConnectionHandler) GetConnectionStatus(ctx context.Context, request *connect.Request[octantv1alpha.GetConnectionStatusRequest]) (*connect.Response[octantv1alpha.GetConnectionStatusResponse], error) {
	connectionStatus, err := ch.octantConnection.GetConnectionStatus(ctx, request.Msg.Namespace, request.Msg.ConnectionName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(connectionStatus), nil
}
