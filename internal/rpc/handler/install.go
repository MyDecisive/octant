package rpchandler

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/integration"
	"sigs.k8s.io/yaml"
	"time"

	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
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
	connectionName := req.Msg.GetConnectionName()

	logger := zap.L().With(zap.String("installNamespace", installNamespace))

	logger.Debug("received install MDAIHub request")

	// 1) get the argo integration details
	argoIntegration, err := ih.argoIntegration.GetIntegrationByName(ctx, ih.config.CurrentNamespace, connectionName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if argoIntegration == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no ArgoCD integration found with name '%s'", connectionName))
	}

	// 2) render the argo app template(s)
	manifestBytes, err := connection.RenderMdaiAppManifest("0.9.3-envoy", installNamespace)
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

func (*InstallHandler) GetInstallStatus(
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
	time.Sleep(3 * time.Second) // nolint: mnd

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
