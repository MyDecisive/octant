package rpchandler

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/samber/lo"

	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
	"sigs.k8s.io/yaml"
)

var terminalInstallStates = []octantv1alpha.InstallStatus{
	octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR,
	octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED,
}

type InstallHandler struct {
	octantv1alphaconnect.UnimplementedInstallServiceHandler

	config          *config.Configuration
	argoClient      argocd.APIClient
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData]
}

func NewInstallHandler(
	theConfig *config.Configuration,
	argoClient argocd.APIClient,
	argoIntegration integration.Integration[integration.ArgoCDIntegrationData],
) *InstallHandler {
	return &InstallHandler{
		config:          theConfig,
		argoClient:      argoClient,
		argoIntegration: argoIntegration,
	}
}

func (ih *InstallHandler) InstallMDAIHub(
	ctx context.Context,
	req *connect.Request[octantv1alpha.InstallMDAIHubRequest],
) (*connect.Response[emptypb.Empty], error) {
	installNamespace := req.Msg.GetNamespace()
	connectionName := req.Msg.GetConnectionName()
	installVersion := req.Msg.GetMdaiVersion()

	logger := zap.L().With(zap.String("installNamespace", installNamespace))

	logger.Debug("received install MDAIHub request")

	// 1) get the argo integration details
	argoIntegration, err := ih.argoIntegration.GetIntegrationByName(ctx, ih.config.CurrentNamespace, connectionName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if argoIntegration == nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	// 2) render the argo app template(s)
	manifestBytes, err := connection.RenderMdaiAppManifest(installVersion, installNamespace)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var argoApp argoapp.Application
	if err = yaml.Unmarshal(manifestBytes, &argoApp); err != nil {
		logger.Error("unmarshalling ArgoCD application manifest", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var certManagerApp argoapp.Application
	if err = yaml.Unmarshal(connection.CertManagerAppManifest, &certManagerApp); err != nil {
		logger.Error("unmarshalling cert manager application manifest", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// 3) apply the application to the argo cluster
	clientOpts := &apiclient.ClientOptions{
		ServerAddr: argoIntegration.APIUrl,
		AuthToken:  argoIntegration.AccountToken,
		Insecure:   ih.config.Env == config.Dev, // ignore certs in localdev
	}
	// first, apply the cert manager app manifest
	logger.Debug("pushing cert-manager app install")
	if err = ih.argoClient.PushArgoApp(ctx, logger, clientOpts, certManagerApp); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logger.Debug("pushing mdai app install")
	if err = ih.argoClient.PushArgoApp(ctx, logger, clientOpts, argoApp); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &connect.Response[emptypb.Empty]{}, nil
}

func (ih *InstallHandler) GetInstallStatus(
	ctx context.Context,
	req *connect.Request[octantv1alpha.GetInstallStatusRequest],
	response *connect.ServerStream[octantv1alpha.GetInstallStatusResponse],
) error {
	connectionName := req.Msg.GetConnectionName()

	logger := zap.L().With(zap.String("connectionName", connectionName))

	logger.Debug("received install status request")

	argoIntegration, err := ih.argoIntegration.GetIntegrationByName(ctx, ih.config.CurrentNamespace, connectionName)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if argoIntegration == nil {
		return connect.NewError(connect.CodeNotFound, err)
	}

	clientOpts := &apiclient.ClientOptions{
		ServerAddr: argoIntegration.APIUrl,
		AuthToken:  argoIntegration.AccountToken,
		Insecure:   ih.config.Env == config.Dev, // ignore certs in localdev
	}
	status, details, err := ih.argoClient.GetAppStatus(ctx, logger, clientOpts)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if status == octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED {
		return connect.NewError(connect.CodeUnknown, errors.New("install status is unspecified"))
	}

	if lo.Contains(terminalInstallStates, status) {
		if err = response.Send(&octantv1alpha.GetInstallStatusResponse{
			InstallStatus: status,
			Details:       details,
		}); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	// we're in a non-terminal state (installing), keep polling for a change
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	shutdownChan := make(chan bool)
	go func() {
		// TODO: configurable timeout??
		time.Sleep(1 * time.Minute)
		shutdownChan <- true
	}()

	for {
		select {
		case <-shutdownChan:
			logger.Warn("reached timeout waiting for app (mdai) to be healthy")
			return connect.NewError(connect.CodeDeadlineExceeded, errors.New("timeout waiting for app (mdai) to be healthy"))
		case <-ticker.C:
			logger.Debug("checking install status")
			status, details, err = ih.argoClient.GetAppStatus(ctx, logger, clientOpts)
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
			if lo.Contains(terminalInstallStates, status) {
				return nil
			}
		}
	}
}
