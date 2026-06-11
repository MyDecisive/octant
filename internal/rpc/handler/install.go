package rpchandler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection/manifest"
	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

type InstallHandler struct {
	octantv1alphaconnect.UnimplementedInstallServiceHandler

	config          *config.Configuration
	argoClient      argocd.APIClient
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData]
	manifestMapper  manifestdata.Mapper
	manifestManager manifest.Manager
}

func NewInstallHandler(
	theConfig *config.Configuration,
	argoClient argocd.APIClient,
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData],
	manifestMapper manifestdata.Mapper,
	manifestManager manifest.Manager,
) *InstallHandler {
	return &InstallHandler{
		config:          theConfig,
		argoClient:      argoClient,
		argoIntegration: argoIntegration,
		manifestMapper:  manifestMapper,
		manifestManager: manifestManager,
	}
}

func (ih *InstallHandler) InstallMDAIHub(
	ctx context.Context,
	req *connect.Request[octantv1alpha.InstallMDAIHubRequest],
) (*connect.Response[emptypb.Empty], error) {
	installNamespace := req.Msg.GetNamespace()
	connectionName := req.Msg.GetConnectionName()
	installVersion := req.Msg.GetMdaiVersion()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.InstallServiceInstallMDAIHubProcedure),
		zap.String("namespace", installNamespace),
		zap.String("mdaiVersion", installVersion),
	)

	logger.Debug("received request")

	input := manifest.ManagerInput{
		Logger:                    logger,
		DeploymentIntegrationName: connectionName,
	}

	logger.Debug("install cert manager")
	if err := ih.manifestManager.LoadCertManager(
		ctx,
		input,
		ih.manifestMapper.AppTemplateData(manifestdata.CERT, "", "", ""),
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("install cert manager:%w", err))
	}

	logger.Debug("install MDAI")
	if err := ih.manifestManager.LoadMDAI(
		ctx,
		input,
		ih.manifestMapper.AppTemplateData(manifestdata.MDAI, installVersion, "", installNamespace),
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("install MDAI:%w", err))
	}

	logger.Debug("install done")
	return &connect.Response[emptypb.Empty]{}, nil
}

func (ih *InstallHandler) GetInstallStatus(
	ctx context.Context,
	req *connect.Request[octantv1alpha.GetInstallStatusRequest],
	response *connect.ServerStream[octantv1alpha.GetInstallStatusResponse],
) error {
	connectionName := req.Msg.GetConnectionName()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.InstallServiceGetInstallStatusProcedure),
		zap.String("connectionName", connectionName),
	)

	logger.Debug("received install status request")

	argoIntegration, err := ih.argoIntegration.GetIntegrationByName(ctx, connectionName)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if argoIntegration == nil {
		return connect.NewError(connect.CodeNotFound, errors.New("argo integration not found"))
	}

	clientOpts := argocd.CreateClientOpts(ih.config.Env, argoIntegration.APIUrl, argoIntegration.AccountToken)
	input := argocd.Input{
		Logger:     logger,
		ClientOpts: clientOpts,
		AppName:    "mdai",
	}
	status, details, err := ih.argoClient.GetAppStatus(ctx, input)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// send the install status
	if err = response.Send(&octantv1alpha.GetInstallStatusResponse{
		InstallStatus: status,
		Details:       details,
	}); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if status == octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED {
		return nil
	}

	// we're in a non-terminal state (installing), keep polling for a change
	ticker := time.NewTicker(time.Duration(ih.config.Install.MdaiInstallPollingIntervalMillis) * time.Millisecond)
	defer ticker.Stop()

	timeoutChan := time.After(time.Duration(ih.config.Install.MdaiInstallTimeout) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return connect.NewError(connect.CodeCanceled, ctx.Err())
		case <-timeoutChan:
			logger.Warn("reached timeout waiting for app (mdai) to be healthy")
			if err = response.Send(&octantv1alpha.GetInstallStatusResponse{
				InstallStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_TIMEOUT,
				Details:       details,
			}); err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			return nil
		case <-ticker.C:
			logger.Debug("checking install status")
			status, details, err = ih.argoClient.GetAppStatus(ctx, input)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			// send the install status
			if err = response.Send(&octantv1alpha.GetInstallStatusResponse{
				InstallStatus: status,
				Details:       details,
			}); err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if status == octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED {
				return nil
			}
			logger.Debug("install still in progress or erroring", zap.String("status", status.String()))
		}
	}
}
