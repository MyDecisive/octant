package argocd

import (
	"context"
	"errors"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type APIClient interface {
	TestConnection(
		ctx context.Context,
		logger *zap.Logger,
		clientOpts *apiclient.ClientOptions,
	) (bool, error)
	PushArgoApp(
		ctx context.Context,
		logger *zap.Logger,
		clientOpts *apiclient.ClientOptions,
		argoApp argoapp.Application,
	) error
	GetAppStatus(
		ctx context.Context,
		logger *zap.Logger,
		clientOpts *apiclient.ClientOptions,
	) (octantv1alpha.InstallStatus, []*octantv1alpha.ResourceDetails, error)
}

type Client struct{}

func NewArgoCDClient() *Client {
	return &Client{}
}

// TestConnection checks the provided clientOpts are valid argo cd API credentials.
func (*Client) TestConnection(
	ctx context.Context,
	logger *zap.Logger,
	clientOpts *apiclient.ClientOptions,
) (bool, error) {
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
		if rpcStatus, isRPCError := status.FromError(err); isRPCError && rpcStatus.Code() == codes.Unauthenticated {
			return false, nil // not an error, creds didn't auth properly.
		}
		logger.Error("getting argo application list", zap.Error(err))
		return false, err
	}
	return true, nil
}

// PushArgoApp creates (upsert if exists) the provided argo application on the argo cluster.
func (*Client) PushArgoApp(
	ctx context.Context,
	logger *zap.Logger,
	clientOpts *apiclient.ClientOptions,
	argoApp argoapp.Application,
) error {
	argoClient, err := apiclient.NewClient(clientOpts)
	if err != nil {
		logger.Error("creating argo api client", zap.Error(err))
		return err
	}

	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		logger.Error("creating argo application client", zap.Error(err))
		return err
	}
	defer func() {
		if err = closer.Close(); err != nil {
			logger.Warn("closing argo api client", zap.Error(err))
		}
	}()

	if _, err = applicationClient.Create(ctx, &application.ApplicationCreateRequest{
		Application: &argoApp,
		Upsert:      lo.ToPtr(true),
	}); err != nil {
		logger.Error("creating argo app", zap.Error(err))
		return err
	}
	return nil
}

// GetAppStatus retrieves the argo application status and any resource details available for a non-healthy state.
func (*Client) GetAppStatus(
	ctx context.Context,
	logger *zap.Logger,
	clientOpts *apiclient.ClientOptions,
) (
	octantv1alpha.InstallStatus,
	[]*octantv1alpha.ResourceDetails,
	error,
) {
	argoClient, err := apiclient.NewClient(clientOpts)
	if err != nil {
		logger.Error("creating argo api client", zap.Error(err))
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, nil, err
	}
	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		logger.Error("creating argo application client", zap.Error(err))
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, nil, err
	}
	defer func() {
		if err = closer.Close(); err != nil {
			logger.Warn("closing argo api client", zap.Error(err))
		}
	}()

	var resourceDetails []*octantv1alpha.ResourceDetails
	argoApp, err := applicationClient.Get(ctx, &application.ApplicationQuery{
		Name: lo.ToPtr("mdai"),
	})
	appHealth := argoApp.Status.Health.Status

	// if the app is healthy, we won't bother pulling the resource tree details
	if appHealth == health.HealthStatusHealthy {
		return healthStatusCodeToAppResourceHealth(appHealth), resourceDetails, nil
	}

	// NOTE about using `ResourceTree` here:
	// 	The application `Get` doesn't retrieve the pods created for the application, just the Deployments
	//	and a list of other resources, which wasn't enough to get individual resource details for why the
	//	application might be in the Installing or Errored state.
	//
	// Also, ideally we can set the `Kind` to "Pod" on the `ResourcesQuery` here and significantly filter down
	// the number of resources coming back, but apparently the `ApplicationName` and `Kind` parameters are
	// mutually exclusive.
	tree, err := applicationClient.ResourceTree(ctx, &application.ResourcesQuery{
		ApplicationName: lo.ToPtr("mdai"),
	})
	if err != nil {
		logger.Error("getting argo application resource tree", zap.Error(err))
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, nil, err
	}

	pods := lo.Filter(tree.Nodes, func(item argoapp.ResourceNode, index int) bool {
		return item.Kind == "Pod"
	})
	if len(pods) == 0 {
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, nil, errors.New("no pod resources found")
	}

	resourceDetails = make([]*octantv1alpha.ResourceDetails, len(pods))
	for i, pod := range pods {
		resourceDetails[i] = &octantv1alpha.ResourceDetails{
			Name:    pod.Name,
			Message: pod.Health.Message,
		}
	}

	return healthStatusCodeToAppResourceHealth(appHealth), resourceDetails, nil
}

func healthStatusCodeToAppResourceHealth(healthStatus health.HealthStatusCode) octantv1alpha.InstallStatus {
	switch healthStatus {
	case health.HealthStatusDegraded:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR
	case health.HealthStatusProgressing, health.HealthStatusMissing, health.HealthStatusUnknown:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING
	case health.HealthStatusHealthy:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED
	default:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED
	}
}
