package argocd

import (
	"net"
	"testing"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	applicationmock "github.com/mydecisive/octant/internal/mock/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTestConnection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description     string
		setupMockServer func(t *testing.T) application.ApplicationServiceServer
		validateResult  func(success bool, err error)
	}{
		{
			description: "unknown error listing applications",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().List(mock.Anything, mock.MatchedBy(func(req *application.ApplicationQuery) bool {
					return req.GetName() == "mdai"
				})).Return(nil, assert.AnError).Once()
				return mockAppServer
			},
			validateResult: func(success bool, err error) {
				require.Error(t, err)
				require.False(t, success)
			},
		},
		{
			description: "happy path - unauthenticated",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().List(mock.Anything, mock.MatchedBy(func(req *application.ApplicationQuery) bool {
					return req.GetName() == "mdai"
				})).Return(nil, status.Error(codes.Unauthenticated, assert.AnError.Error())).Once()
				return mockAppServer
			},
			validateResult: func(success bool, err error) {
				require.NoError(t, err)
				require.False(t, success)
			},
		},
		{
			description: "happy path",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().List(mock.Anything, mock.MatchedBy(func(req *application.ApplicationQuery) bool {
					return req.GetName() == "mdai"
				})).Return(&v1alpha1.ApplicationList{}, nil).Once()
				return mockAppServer
			},
			validateResult: func(success bool, err error) {
				require.NoError(t, err)
				require.True(t, success)
			},
		},
	}

	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			lc := &net.ListenConfig{
				KeepAlive: 5 * time.Second,
			}
			lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)

			s := grpc.NewServer()

			mockAppServer := testCase.setupMockServer(t)

			application.RegisterApplicationServiceServer(s, mockAppServer)

			// start the mock app service server
			go s.Serve(lis) // nolint: errcheck

			t.Cleanup(s.Stop)

			clientOpts := &apiclient.ClientOptions{
				ServerAddr: lis.Addr().String(),
				Insecure:   true,
				PlainText:  true, // needed for local testing
			}

			testClient := NewArgoCDClient()
			success, testErr := testClient.TestConnection(t.Context(), zaptest.NewLogger(t), clientOpts)
			testCase.validateResult(success, testErr)
		})
	}
}

func TestPushArgoApp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description     string
		testApp         v1alpha1.Application
		setupMockServer func(t *testing.T) application.ApplicationServiceServer
		validateResult  func(err error)
	}{
		{
			description: "unknown error creating application",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Create(mock.Anything, mock.MatchedBy(func(req *application.ApplicationCreateRequest) bool {
					return req.GetUpsert() && req.GetApplication() != nil
				})).Return(nil, assert.AnError).Once()
				return mockAppServer
			},
			validateResult: func(err error) {
				require.Error(t, err)
			},
		},
		{
			description: "happy path",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Create(mock.Anything, mock.MatchedBy(func(req *application.ApplicationCreateRequest) bool {
					return req.GetUpsert() && req.GetApplication() != nil
				})).Return(&v1alpha1.Application{}, nil).Once()
				return mockAppServer
			},
			validateResult: func(err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			lc := &net.ListenConfig{
				KeepAlive: 5 * time.Second,
			}
			lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)

			s := grpc.NewServer()

			mockAppServer := testCase.setupMockServer(t)

			application.RegisterApplicationServiceServer(s, mockAppServer)

			// start the mock app service server
			go s.Serve(lis) // nolint: errcheck

			t.Cleanup(s.Stop)

			clientOpts := &apiclient.ClientOptions{
				ServerAddr: lis.Addr().String(),
				Insecure:   true,
				PlainText:  true, // needed for local testing
			}

			testClient := NewArgoCDClient()
			testErr := testClient.PushArgoApp(t.Context(), zaptest.NewLogger(t), clientOpts, testCase.testApp)
			testCase.validateResult(testErr)
		})
	}
}

func TestGetAppStatus(t *testing.T) {
	t.Parallel()

	appTreeNoPods := &v1alpha1.ApplicationTree{
		Nodes: []v1alpha1.ResourceNode{
			{
				ResourceRef: v1alpha1.ResourceRef{
					Kind: "Deployment",
				},
			},
			{
				ResourceRef: v1alpha1.ResourceRef{
					Kind: "ConfigMap",
				},
			},
		},
	}

	appTreeWithPods := &v1alpha1.ApplicationTree{
		Nodes: []v1alpha1.ResourceNode{
			{
				ResourceRef: v1alpha1.ResourceRef{
					Kind: "Pod",
					Name: "coolPod",
				},
				Health: &v1alpha1.HealthStatus{
					Status:  health.HealthStatusProgressing,
					Message: "chill out, pod is still bootstrapping",
				},
			},
			{
				ResourceRef: v1alpha1.ResourceRef{
					Kind: "Pod",
					Name: "otherCoolPod",
				},
				Health: &v1alpha1.HealthStatus{
					Status:  health.HealthStatusHealthy,
					Message: "OK",
				},
			},
			{
				ResourceRef: v1alpha1.ResourceRef{
					Kind: "Deployment",
				},
			},
			{
				ResourceRef: v1alpha1.ResourceRef{
					Kind: "ConfigMap",
				},
			},
		},
	}

	healthyApp := &v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{
			Health: v1alpha1.AppHealthStatus{
				Status: health.HealthStatusHealthy,
			},
		},
	}

	unhealthyApp := &v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{
			Health: v1alpha1.AppHealthStatus{
				Status: health.HealthStatusDegraded,
			},
		},
	}

	testCases := []struct {
		description     string
		testApp         v1alpha1.Application
		setupMockServer func(t *testing.T) application.ApplicationServiceServer
		validateResult  func(octantv1alpha.InstallStatus, []*octantv1alpha.ResourceDetails, error)
	}{
		{
			description: "unknown error getting argo app",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Get(mock.Anything, mock.MatchedBy(func(req *application.ApplicationQuery) bool {
					return req.GetName() == "mdai"
				})).Return(nil, assert.AnError).Once()
				return mockAppServer
			},
			validateResult: func(is octantv1alpha.InstallStatus, rd []*octantv1alpha.ResourceDetails, err error) {
				require.Error(t, err)
				assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, is)
				assert.Nil(t, rd)
			},
		},
		{
			description: "healthy app status",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Get(mock.Anything, mock.MatchedBy(func(req *application.ApplicationQuery) bool {
					return req.GetName() == "mdai"
				})).Return(healthyApp, nil).Once()
				return mockAppServer
			},
			validateResult: func(is octantv1alpha.InstallStatus, rd []*octantv1alpha.ResourceDetails, err error) {
				require.NoError(t, err)
				assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED, is)
				assert.Empty(t, rd)
			},
		},
		{
			description: "unknown error getting resource tree",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Get(mock.Anything, mock.MatchedBy(func(req *application.ApplicationQuery) bool {
					return req.GetName() == "mdai"
				})).Return(unhealthyApp, nil).Once()
				mockAppServer.EXPECT().ResourceTree(mock.Anything, mock.MatchedBy(func(req *application.ResourcesQuery) bool {
					return req.GetApplicationName() == "mdai"
				})).Return(nil, assert.AnError).Once()
				return mockAppServer
			},
			validateResult: func(is octantv1alpha.InstallStatus, rd []*octantv1alpha.ResourceDetails, err error) {
				require.Error(t, err)
				assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED, is)
				assert.Nil(t, rd)
			},
		},
		{
			description: "no pod resources",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Get(mock.Anything, mock.MatchedBy(func(req *application.ApplicationQuery) bool {
					return req.GetName() == "mdai"
				})).Return(unhealthyApp, nil).Once()
				mockAppServer.EXPECT().ResourceTree(mock.Anything, mock.MatchedBy(func(req *application.ResourcesQuery) bool {
					return req.GetApplicationName() == "mdai"
				})).Return(appTreeNoPods, nil).Once()
				return mockAppServer
			},
			validateResult: func(is octantv1alpha.InstallStatus, rd []*octantv1alpha.ResourceDetails, err error) {
				require.NoError(t, err)
				require.Empty(t, rd)
				assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING, is)
			},
		},
		{
			description: "errored app status with details",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Get(mock.Anything, mock.MatchedBy(func(req *application.ApplicationQuery) bool {
					return req.GetName() == "mdai"
				})).Return(unhealthyApp, nil).Once()
				mockAppServer.EXPECT().ResourceTree(mock.Anything, mock.MatchedBy(func(req *application.ResourcesQuery) bool {
					return req.GetApplicationName() == "mdai"
				})).Return(appTreeWithPods, nil).Once()
				return mockAppServer
			},
			validateResult: func(is octantv1alpha.InstallStatus, rd []*octantv1alpha.ResourceDetails, err error) {
				require.NoError(t, err)

				require.Len(t, rd, 2)
				assert.Equal(t, "coolPod", rd[0].GetName())
				assert.Equal(t, "chill out, pod is still bootstrapping", rd[0].GetMessage())
				assert.Equal(t, "otherCoolPod", rd[1].GetName())
				assert.Equal(t, "OK", rd[1].GetMessage())

				require.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR, is)
			},
		},
	}

	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			lc := &net.ListenConfig{
				KeepAlive: 5 * time.Second,
			}
			lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)

			s := grpc.NewServer()

			mockAppServer := testCase.setupMockServer(t)

			application.RegisterApplicationServiceServer(s, mockAppServer)

			// start the mock app service server
			go s.Serve(lis) // nolint: errcheck

			t.Cleanup(s.Stop)

			clientOpts := &apiclient.ClientOptions{
				ServerAddr: lis.Addr().String(),
				Insecure:   true,
				PlainText:  true, // needed for local testing
			}

			testClient := NewArgoCDClient()
			installStatus, resourceDetails, testErr := testClient.GetAppStatus(t.Context(), zaptest.NewLogger(t), clientOpts)
			testCase.validateResult(installStatus, resourceDetails, testErr)
		})
	}
}

func TestHealthStatusCodeToAppResourceHealth(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description          string
		inputStatus          health.HealthStatusCode
		expectedOctantStatus octantv1alpha.InstallStatus
	}{
		{
			description:          "suspended",
			inputStatus:          health.HealthStatusSuspended,
			expectedOctantStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_UNSPECIFIED,
		},
		{
			description:          "degraded",
			inputStatus:          health.HealthStatusDegraded,
			expectedOctantStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR,
		},
		{
			description:          "missing",
			inputStatus:          health.HealthStatusMissing,
			expectedOctantStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING,
		},
		{
			description:          "unknown",
			inputStatus:          health.HealthStatusUnknown,
			expectedOctantStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING,
		},
		{
			description:          "progressing",
			inputStatus:          health.HealthStatusProgressing,
			expectedOctantStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING,
		},
		{
			description:          "healthy",
			inputStatus:          health.HealthStatusHealthy,
			expectedOctantStatus: octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			octantStatus := healthStatusCodeToAppResourceHealth(tc.inputStatus)
			assert.Equal(t, tc.expectedOctantStatus, octantStatus)
		})
	}
}

func TestDeleteArgoApp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description     string
		testApp         v1alpha1.Application
		setupMockServer func(t *testing.T) application.ApplicationServiceServer
		validateResult  func(err error)
	}{
		{
			description: "unknown error deleting application",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Delete(mock.Anything, mock.MatchedBy(func(req *application.ApplicationDeleteRequest) bool {
					return req.GetName() == "mdai" &&
						req.GetAppNamespace() == "argocd" &&
						req.GetCascade() &&
						req.GetPropagationPolicy() == "foreground"
				})).Return(nil, assert.AnError).Once()
				return mockAppServer
			},
			validateResult: func(err error) {
				require.Error(t, err)
			},
		},
		{
			description: "happy path",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Delete(mock.Anything, mock.MatchedBy(func(req *application.ApplicationDeleteRequest) bool {
					return req.GetName() == "mdai" &&
						req.GetAppNamespace() == "argocd" &&
						req.GetCascade() &&
						req.GetPropagationPolicy() == "foreground"
				})).Return(&application.ApplicationResponse{}, nil).Once()
				return mockAppServer
			},
			validateResult: func(err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			lc := &net.ListenConfig{
				KeepAlive: 5 * time.Second,
			}
			lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)

			s := grpc.NewServer()

			mockAppServer := testCase.setupMockServer(t)

			application.RegisterApplicationServiceServer(s, mockAppServer)

			// start the mock app service server
			go s.Serve(lis) // nolint: errcheck

			t.Cleanup(s.Stop)

			clientOpts := &apiclient.ClientOptions{
				ServerAddr: lis.Addr().String(),
				Insecure:   true,
				PlainText:  true, // needed for local testing
			}

			testClient := NewArgoCDClient()
			testErr := testClient.DeleteArgoApp(t.Context(), zaptest.NewLogger(t), clientOpts, "mdai")
			testCase.validateResult(testErr)
		})
	}
}

func TestSyncApplication(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description     string
		testApp         v1alpha1.Application
		setupMockServer func(t *testing.T) application.ApplicationServiceServer
		validateResult  func(err error)
	}{
		{
			description: "unknown error syncing application",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Sync(mock.Anything, mock.MatchedBy(func(req *application.ApplicationSyncRequest) bool {
					return req.GetName() == "mdai" &&
						req.GetRevision() == "HEAD" &&
						!req.GetPrune() &&
						!req.GetDryRun() &&
						req.GetStrategy().Apply.Force &&
						len(req.GetManifests()) == 2
				})).Return(nil, assert.AnError).Once()
				return mockAppServer
			},
			validateResult: func(err error) {
				require.Error(t, err)
			},
		},
		{
			description: "happy path",
			setupMockServer: func(t *testing.T) application.ApplicationServiceServer {
				t.Helper()
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Sync(mock.Anything, mock.MatchedBy(func(req *application.ApplicationSyncRequest) bool {
					return req.GetName() == "mdai" &&
						req.GetRevision() == "HEAD" &&
						!req.GetPrune() &&
						!req.GetDryRun() &&
						req.GetStrategy().Apply.Force &&
						len(req.GetManifests()) == 2
				})).Return(&v1alpha1.Application{}, nil).Once()
				return mockAppServer
			},
			validateResult: func(err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			lc := &net.ListenConfig{
				KeepAlive: 5 * time.Second,
			}
			lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)

			s := grpc.NewServer()

			mockAppServer := testCase.setupMockServer(t)

			application.RegisterApplicationServiceServer(s, mockAppServer)

			// start the mock app service server
			go s.Serve(lis) // nolint: errcheck

			t.Cleanup(s.Stop)

			clientOpts := &apiclient.ClientOptions{
				ServerAddr: lis.Addr().String(),
				Insecure:   true,
				PlainText:  true, // needed for local testing
			}

			testClient := NewArgoCDClient()
			testErr := testClient.SyncApplication(
				t.Context(),
				zaptest.NewLogger(t),
				clientOpts,
				"mdai",
				[]string{"manifest 1 content", "manifest 2 content"},
				false,
			)
			testCase.validateResult(testErr)
		})
	}
}
