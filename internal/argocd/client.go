package argocd

import (
	"context"
	"errors"
	"fmt"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argoclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/mydecisive/octant/internal/config"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	clientRetryMax = 3
)

var (
	ErrWatch   = errors.New("watch")
	ErrDeleted = errors.New("deleted")
	ErrParsing = errors.New("parsing")
)

type Input struct {
	Logger     *zap.Logger
	ClientOpts *apiclient.ClientOptions
	AppName    string
}

type InstallResult struct {
	Status octantv1alpha.InstallStatus
	Err    error
}

type WatchResult struct {
	Details *argoapp.Application
	Err     error
}

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
	DeleteArgoApp(
		ctx context.Context,
		input Input,
	) error
	WatchApplication(
		ctx context.Context,
		input Input,
		timeout time.Duration,
		out chan WatchResult,
	)
	// AppOperationState continuously retrieves the app operation state until
	// either the timeout is reached or the state is succeeded.
	AppOperationState(
		ctx context.Context,
		input Input,
		interval time.Duration,
		timeout time.Duration,
		out chan InstallResult,
	)
	// WaitForAppOperation blocks until the application has no sync operation in flight (the current
	// operation reached a terminal phase, or there is none).
	WaitForAppOperation(
		ctx context.Context,
		input Input,
		interval time.Duration,
		timeout time.Duration,
	) error
	GetAppStatus(
		ctx context.Context,
		input Input,
	) (octantv1alpha.InstallStatus, []*octantv1alpha.ResourceDetails, error)
	SyncApplication(
		ctx context.Context,
		input Input,
		manifests []string,
		prune bool,
	) error
}

type Client struct {
	argokube  *argoclientset.Clientset
	appConfig *config.Configuration
}

// Ensure Client implements APIClient.
var _ APIClient = (*Client)(nil)

func NewArgoCDClient(appConfig *config.Configuration, argokube *argoclientset.Clientset) *Client {
	return &Client{
		appConfig: appConfig,
		argokube:  argokube,
	}
}

// CreateClientOpts create a new ArgoCD client option to be passed
// to the ArgoCD client methods.
func CreateClientOpts(env config.Environment, clusterURL, authToken string) *apiclient.ClientOptions {
	return &apiclient.ClientOptions{
		HttpRetryMax: clientRetryMax,
		ServerAddr:   clusterURL,
		AuthToken:    authToken,
		Insecure:     env == config.Dev, // ignore certs in localdev
	}
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

func (c *Client) DeleteArgoApp(
	ctx context.Context,
	input Input,
) error {
	argoClient, err := apiclient.NewClient(input.ClientOpts)
	if err != nil {
		input.Logger.Error("creating argo api client", zap.Error(err))
		return err
	}
	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		input.Logger.Error("creating argo application client", zap.Error(err))
		return err
	}
	defer func() {
		if err = closer.Close(); err != nil {
			input.Logger.Warn("closing argo api client", zap.Error(err))
		}
	}()
	if _, err = applicationClient.Delete(ctx, &application.ApplicationDeleteRequest{
		Name:              lo.ToPtr(input.AppName),
		AppNamespace:      lo.ToPtr(c.appConfig.Install.ArgoCDNamespace),
		Cascade:           lo.ToPtr(true),
		PropagationPolicy: lo.ToPtr("foreground"),
	}); err != nil {
		input.Logger.Error("deleting argo app", zap.Error(err))
		return err
	}
	return nil
}

func (*Client) SyncApplication(
	ctx context.Context,
	input Input,
	manifests []string,
	prune bool,
) error {
	argoClient, err := apiclient.NewClient(input.ClientOpts)
	if err != nil {
		input.Logger.Error("creating argo api client", zap.Error(err))
		return err
	}
	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		input.Logger.Error("creating argo application client", zap.Error(err))
		return err
	}
	defer func() {
		if err = closer.Close(); err != nil {
			input.Logger.Warn("closing argo api client", zap.Error(err))
		}
	}()

	if _, err = applicationClient.Sync(ctx, &application.ApplicationSyncRequest{
		Name:     lo.ToPtr(input.AppName),
		Revision: lo.ToPtr("HEAD"),
		Prune:    lo.ToPtr(prune),
		DryRun:   lo.ToPtr(false),
		Strategy: &argoapp.SyncStrategy{
			Apply: &argoapp.SyncStrategyApply{
				Force: true,
			},
		},
		Manifests: manifests,
	}); err != nil {
		input.Logger.Error("syncing argo application", zap.Error(err))
		return err
	}
	return nil
}

func (c *Client) WatchApplication(
	ctx context.Context,
	input Input,
	timeout time.Duration,
	out chan WatchResult,
) {
	defer close(out) // Tell caller the operation is complete

	apps, err := c.argokube.ArgoprojV1alpha1().Applications(c.appConfig.Install.ArgoCDNamespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: "app = mdai",
		},
	)
	if err != nil {
		out <- WatchResult{
			Err: fmt.Errorf("%w:%w", ErrWatch, err),
		}
		return
	}
	resourceVer := apps.ResourceVersion
	if len(apps.Items) > 0 {
		out <- WatchResult{
			Details: &apps.Items[0],
		}
	}

	// labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": input.AppName}}
	// TODO: use argocd app namespace
	watcher, err := c.argokube.ArgoprojV1alpha1().Applications(c.appConfig.Install.ArgoCDNamespace).
		Watch(ctx, metav1.ListOptions{
			LabelSelector:       "app = mdai",
			TimeoutSeconds:      lo.ToPtr(int64(timeout.Seconds())),
			ResourceVersion:     resourceVer,
			AllowWatchBookmarks: true,
		})
	if err != nil {
		out <- WatchResult{
			Err: fmt.Errorf("%w:%w", ErrWatch, err),
		}
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			input.Logger.Debug("Context cancelled, end watch")
			return
		case event := <-watcher.ResultChan():
			switch event.Type {
			case watch.Modified, watch.Added:
				app, ok := event.Object.(*argoapp.Application)
				if !ok {
					out <- WatchResult{
						Err: ErrParsing,
					}
					continue
				}
				out <- WatchResult{
					Details: app,
				}
			case watch.Deleted:
				out <- WatchResult{
					Err: ErrDeleted,
				}
			case watch.Error:
				input.Logger.Error("Received errored event type", zap.Any("details", event.Object)) //nolint
				out <- WatchResult{
					Err: errors.New("unexpected"),
				}
			default:
				input.Logger.Debug("Ignore event type", zap.String("type", string(event.Type)), zap.Any("details", event.Object)) //nolint
			}
		}
	}
}

// pollAppOperation polls the app's operation state until done is satisfied or the timeout elapses.
// onState, when non-nil, observes the state on each poll for callers that stream progress.
func pollAppOperation(
	ctx context.Context,
	input Input,
	interval, timeout time.Duration,
	onState func(*argoapp.OperationState),
	done func(*argoapp.OperationState) bool,
) error {
	argoClient, err := apiclient.NewClient(input.ClientOpts)
	if err != nil {
		input.Logger.Error("creating argo api client", zap.Error(err))
		return err
	}
	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		input.Logger.Error("creating argo application client", zap.Error(err))
		return err
	}
	defer func() {
		if err = closer.Close(); err != nil {
			input.Logger.Warn("closing argo api client", zap.Error(err))
		}
	}()

	return wait.PollUntilContextTimeout(ctx, interval, timeout, true,
		func(ctx context.Context) (bool, error) {
			argoApp, getErr := applicationClient.Get(ctx, &application.ApplicationQuery{Name: &input.AppName})
			if getErr != nil {
				input.Logger.Error("getting argo application", zap.Error(getErr))
				return false, getErr
			}
			opState := argoApp.Status.OperationState
			if onState != nil {
				onState(opState)
			}
			return done(opState), nil
		})
}

func (*Client) AppOperationState(
	ctx context.Context,
	input Input,
	interval time.Duration,
	timeout time.Duration,
	out chan InstallResult,
) {
	defer close(out) // Tell caller the operation is complete
	err := pollAppOperation(ctx, input, interval, timeout,
		func(opState *argoapp.OperationState) {
			state := octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING
			if opState != nil && opState.Phase == common.OperationSucceeded {
				state = octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED
			}
			out <- InstallResult{Status: state}
		},
		func(opState *argoapp.OperationState) bool {
			return opState != nil && opState.Phase == common.OperationSucceeded
		})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			out <- InstallResult{Status: octantv1alpha.InstallStatus_INSTALL_STATUS_TIMEOUT}
			return
		}
		out <- InstallResult{
			Status: octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR,
			Err:    err,
		}
	}
}

func (*Client) WaitForAppOperation(
	ctx context.Context,
	input Input,
	interval time.Duration,
	timeout time.Duration,
) error {
	if err := pollAppOperation(ctx, input, interval, timeout, nil,
		func(opState *argoapp.OperationState) bool {
			return opState == nil || opState.Phase.Completed()
		}); err != nil {
		return fmt.Errorf("waiting for argo application %s operation to complete: %w", input.AppName, err)
	}
	return nil
}

// GetAppStatus retrieves the argo application status and any resource details available for a non-healthy state.
func (*Client) GetAppStatus(
	ctx context.Context,
	input Input,
) (
	octantv1alpha.InstallStatus,
	[]*octantv1alpha.ResourceDetails,
	error,
) {
	input.Logger = input.Logger.With(zap.String("appName", input.AppName))
	name := lo.ToPtr(input.AppName)
	argoClient, err := apiclient.NewClient(input.ClientOpts)
	if err != nil {
		input.Logger.Error("creating argo api client", zap.Error(err))
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, nil, err
	}
	closer, applicationClient, err := argoClient.NewApplicationClient()
	if err != nil {
		input.Logger.Error("creating argo application client", zap.Error(err))
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, nil, err
	}
	defer func() {
		if err = closer.Close(); err != nil {
			input.Logger.Warn("closing argo api client", zap.Error(err))
		}
	}()

	var resourceDetails []*octantv1alpha.ResourceDetails
	argoApp, err := applicationClient.Get(ctx, &application.ApplicationQuery{
		Name: name,
	})
	if err != nil {
		input.Logger.Error("getting argo application", zap.Error(err))
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, nil, err
	}
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
		ApplicationName: name,
	})
	if err != nil {
		input.Logger.Error("getting argo application resource tree", zap.Error(err))
		return octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, resourceDetails, err
	}

	pods := lo.Filter(tree.Nodes, func(item argoapp.ResourceNode, index int) bool {
		return item.Kind == "Pod"
	})
	if len(pods) == 0 {
		input.Logger.Debug("no pods found (yet)")
		return octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING, resourceDetails, nil
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
