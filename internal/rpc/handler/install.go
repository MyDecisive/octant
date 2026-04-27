// Package rpchandler contains handlers that will handle RPC service calls.
package rpchandler

import (
	"context"
	"time"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

type InstallHandler struct {
	octantv1alphaconnect.UnimplementedInstallServiceHandler
}

func NewInstallHandler() *InstallHandler {
	return &InstallHandler{}
}

func (ih *InstallHandler) InstallMDAIHub(
	_ context.Context,
	req *connect.Request[octantv1alpha.InstallMDAIHubRequest],
) (*connect.Response[emptypb.Empty], error) {
	installNamespace := req.Msg.GetNamespace()

	logger := zap.L().With(zap.String("installNamespace", installNamespace))

	logger.Debug("received install MDAIHub request")
	return &connect.Response[emptypb.Empty]{}, nil
}

func (ih *InstallHandler) GetInstallStatus(
	_ context.Context,
	req *connect.Request[octantv1alpha.GetInstallStatusRequest],
	response *connect.ServerStream[octantv1alpha.GetInstallStatusResponse],
) error {
	hubName := req.Msg.GetHubName()

	logger := zap.L().With(zap.String("hubName", hubName))

	logger.Debug("received install status request")

	err := response.Send(&octantv1alpha.GetInstallStatusResponse{
		InstallStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING,
		Details:       "installing...",
	})
	if err != nil {
		logger.Error("sending install status", zap.Error(err))
		return connect.NewError(connect.CodeInternal, err)
	}

	// for now, emulating install time passing...
	time.Sleep(3 * time.Second)

	err = response.Send(&octantv1alpha.GetInstallStatusResponse{
		InstallStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED,
		Details:       "successfully installed",
	})
	if err != nil {
		logger.Error("sending install status", zap.Error(err))
		return connect.NewError(connect.CodeInternal, err)
	}
	return nil
}
