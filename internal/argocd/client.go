package argocd

import (
	"context"

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

func (*Client) GetAppStatus(
	ctx context.Context,
	logger *zap.Logger,
	clientOpts *apiclient.ClientOptions,
) (octantv1alpha.InstallStatus, []*octantv1alpha.ResourceDetails, error) {
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

	// NOTE about using `ResourceTree` here instead of just `GetApplication`:
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

	resourceDetails := make([]*octantv1alpha.ResourceDetails, len(pods))
	statuses := make([]octantv1alpha.InstallStatus, len(pods))
	for i, pod := range pods {
		resourceDetails[i] = &octantv1alpha.ResourceDetails{
			Name:    pod.Name,
			Message: pod.Health.Message,
		}
		statuses[i] = healthStatusCodeToAppResourceHealth(pod.Health.Status)
	}

	return determineAppInstallStatus(statuses), resourceDetails, nil
}

func determineAppInstallStatus(statuses []octantv1alpha.InstallStatus) octantv1alpha.InstallStatus {
	numErrored := lo.Count(statuses, octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR)
	numInstalled := lo.Count(statuses, octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED)
	numInstalling := lo.Count(statuses, octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING)

	switch {
	case numErrored == 0 && numInstalling == 0 && numInstalled > 0:
		// no errored or installing resources, so the app is installed
		return octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED
	case numErrored > 0:
		// contains errored resources, the app is errored
		return octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR
	case numInstalling > 0:
		// contains in progress resources, the app is installing
		return octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING
	default:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED
	}
}

func healthStatusCodeToAppResourceHealth(status health.HealthStatusCode) octantv1alpha.InstallStatus {
	switch status {
	case health.HealthStatusDegraded, health.HealthStatusMissing, health.HealthStatusUnknown:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR
	case health.HealthStatusProgressing:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING
	case health.HealthStatusHealthy:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED
	default:
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED
	}
}
