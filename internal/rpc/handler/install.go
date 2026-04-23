// Package rpchandler contains handlers that will handle RPC service calls.
package rpchandler

import (
	"context"
	"fmt"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"google.golang.org/protobuf/types/known/emptypb"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

type InstallHandler struct {
	octantv1alphaconnect.UnimplementedInstallServiceHandler

	config          *config.Configuration
	argoClient      argocd.APIClient
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData]
}

func NewInstallHandler(
	config *config.Configuration,
	argoClient argocd.APIClient,
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData],
) *InstallHandler {
	return &InstallHandler{
		config:          config,
		argoClient:      argoClient,
		argoIntegration: argoIntegration,
	}
}

func (ih *InstallHandler) InstallMDAIHub(
	ctx context.Context,
	req *connect.Request[octantv1alpha.InstallMDAIHubRequest],
) (*connect.Response[emptypb.Empty], error) {
	installNamespace := req.Msg.GetNamespace()

	logger := zap.L().With(zap.String("installNamespace", installNamespace))

	logger.Debug("received install MDAIHub request")

	// get the argo integration details
	argoIntegration, err := ih.argoIntegration.GetIntegrationByName(ctx, ih.config.CurrentNamespace, integration.ArgocdSecretName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if argoIntegration == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no ArgoCD integration found with name '%s'", integration.ArgocdSecretName))
	}

	// TODO: upsert connection configmap with the install namespace
	// create the argo app template, and apply the application to the argo cluster
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
