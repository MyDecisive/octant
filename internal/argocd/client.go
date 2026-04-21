package argocd

import (
	"context"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type APIClient interface {
	TestConnection(ctx context.Context, logger *zap.Logger, clientOpts *apiclient.ClientOptions) bool
}

type ArgoCDClient struct {
}

func NewArgoCDClient() *ArgoCDClient {
	return &ArgoCDClient{}
}

func (a *ArgoCDClient) TestConnection(ctx context.Context, logger *zap.Logger, clientOpts *apiclient.ClientOptions) bool {
	argoClient, err := apiclient.NewClient(clientOpts)
	if err != nil {
		logger.Error("creating argo api client", zap.Error(err))
		return false
	}

	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		logger.Error("creating argo application client", zap.Error(err))
		return false
	}
	defer closer.Close()

	// to validate the account token, we'll query for a list of applications, which requires a valid account token.
	_, err = applicationClient.List(ctx, &application.ApplicationQuery{
		Name: lo.ToPtr("mdai"),
	})
	if err != nil {
		logger.Error("getting argo application list", zap.Error(err))
		return false
	}
	return true
}
