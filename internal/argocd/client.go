package argocd

import (
	"context"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type APIClient interface {
	TestConnection(ctx context.Context, logger *zap.Logger, clientOpts *apiclient.ClientOptions) (bool, error)
}

type ArgoCDClient struct {
}

func NewArgoCDClient() *ArgoCDClient {
	return &ArgoCDClient{}
}

func (a *ArgoCDClient) TestConnection(ctx context.Context, logger *zap.Logger, clientOpts *apiclient.ClientOptions) (bool, error) {
	argoClient, err := apiclient.NewClient(clientOpts)
	if err != nil {
		logger.Error("creating argo api client", zap.Error(err))
		return false, err
	}

	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		logger.Error("creating argo application client", zap.Error(err))
		return false, err
	}
	defer func() {
		if err = closer.Close(); err != nil {
			logger.Warn("closing argo api client", zap.Error(err))
		}
	}()

	// to validate the account token, we'll query for a list of applications, which requires a valid account token.
	_, err = applicationClient.List(ctx, &application.ApplicationQuery{
		Name: lo.ToPtr("mdai"),
	})
	if err != nil {
		rpcStatus, isRPCError := status.FromError(err)
		if isRPCError && rpcStatus.Code() == codes.Unauthenticated {
			return false, nil // not an error, creds didn't auth properly.
		}
		logger.Error("getting argo application list", zap.Error(err))
		return false, err
	}
	return true, nil
}
