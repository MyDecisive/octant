package rpchandler

import (
	"context"
	"errors"
	"maps"
	"slices"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/gitops"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

type GitOpsHandler struct {
	octantv1alphaconnect.UnimplementedGitOpsServiceHandler

	config            *config.Configuration
	githubAppClient   gitops.APIClient
	githubIntegration integration.Integration[integration.GitHubAppIntegrationData]
}

func NewGitOpsHandler(
	configuration *config.Configuration,
	githubAppClient gitops.APIClient,
	githubIntegration integration.Integration[integration.GitHubAppIntegrationData],
) *GitOpsHandler {
	return &GitOpsHandler{
		config:            configuration,
		githubAppClient:   githubAppClient,
		githubIntegration: githubIntegration,
	}
}

func (gh *GitOpsHandler) TestGitHubAppConnection(
	ctx context.Context,
	req *connect.Request[octantv1alpha.TestGitHubAppConnectionRequest],
) (*connect.Response[octantv1alpha.TestGitHubAppConnectionResponse], error) {
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.GitOpsServiceTestGitHubAppConnectionProcedure),
		zap.String("owner", req.Msg.GetOwner()),
		zap.String("repository", req.Msg.GetRepository()),
	)

	logger.Debug("received request")

	success, err := gh.githubAppClient.TestConnection(ctx, integration.GitHubAppIntegrationData{
		AppID:          req.Msg.GetAppId(),
		InstallationID: req.Msg.GetInstallationId(),
		PrivateKey:     req.Msg.GetPrivateKey(),
		Owner:          req.Msg.GetOwner(),
		Repository:     req.Msg.GetRepository(),
	})
	if err != nil {
		logger.Error("testing github app connection", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&octantv1alpha.TestGitHubAppConnectionResponse{
		Success: success,
	}), nil
}

func (gh *GitOpsHandler) SaveGitHubAppConnection(
	ctx context.Context,
	req *connect.Request[octantv1alpha.SaveGitHubAppConnectionRequest],
) (*connect.Response[emptypb.Empty], error) {
	integrationName := req.Msg.GetName()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.GitOpsServiceSaveGitHubAppConnectionProcedure),
		zap.String("integrationName", integrationName),
	)

	logger.Debug("received request")

	privateKey := req.Msg.GetPrivateKey()
	if privateKey == "" {
		existing, err := gh.githubIntegration.GetIntegrationByName(ctx, integrationName)
		if err != nil {
			logger.Error("getting existing integration", zap.Error(err))
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if existing == nil || existing.PrivateKey == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("privateKey is required"))
		}
		privateKey = existing.PrivateKey
	}

	if err := gh.githubIntegration.SetIntegration(ctx, integrationName,
		integration.GitHubAppIntegrationData{
			AppID:          req.Msg.GetAppId(),
			InstallationID: req.Msg.GetInstallationId(),
			PrivateKey:     privateKey,
			Owner:          req.Msg.GetOwner(),
			Repository:     req.Msg.GetRepository(),
			BaseBranch:     req.Msg.GetBaseBranch(),
			BranchPrefix:   req.Msg.GetBranchPrefix(),
			BasePath:       req.Msg.GetBasePath(),
			CommitterName:  req.Msg.GetCommitterName(),
			CommitterEmail: req.Msg.GetCommitterEmail(),
		}); err != nil {
		logger.Error("setting integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &connect.Response[emptypb.Empty]{}, nil
}

func (gh *GitOpsHandler) GetGitHubAppIntegrations(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[octantv1alpha.GetGitHubAppIntegrationsResponse], error) {
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.GitOpsServiceGetGitHubAppIntegrationsProcedure))

	logger.Debug("received request")

	integrations, err := gh.githubIntegration.GetIntegrations(ctx)
	if err != nil {
		logger.Error("getting integrations", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&octantv1alpha.GetGitHubAppIntegrationsResponse{
		Names: slices.Collect(maps.Keys(integrations)),
	}), nil
}

func (gh *GitOpsHandler) GetGitHubAppIntegrationByName(
	ctx context.Context,
	req *connect.Request[octantv1alpha.GetGitHubAppIntegrationByNameRequest],
) (*connect.Response[octantv1alpha.GetGitHubAppIntegrationByNameResponse], error) {
	integrationName := req.Msg.GetName()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.GitOpsServiceGetGitHubAppIntegrationByNameProcedure),
		zap.String("integrationName", integrationName),
	)

	logger.Debug("received request")

	data, err := gh.githubIntegration.GetIntegrationByName(ctx, integrationName)
	if err != nil {
		logger.Error("getting integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if data == nil {
		return connect.NewResponse(&octantv1alpha.GetGitHubAppIntegrationByNameResponse{}), nil
	}

	return connect.NewResponse(&octantv1alpha.GetGitHubAppIntegrationByNameResponse{
		AppId:                data.AppID,
		InstallationId:       data.InstallationID,
		Owner:                data.Owner,
		Repository:           data.Repository,
		BaseBranch:           data.BaseBranch,
		BranchPrefix:         data.BranchPrefix,
		BasePath:             data.BasePath,
		CommitterName:        data.CommitterName,
		CommitterEmail:       data.CommitterEmail,
		PrivateKeyConfigured: data.PrivateKey != "",
	}), nil
}
