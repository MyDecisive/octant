package rpchandler

import (
	"context"
	"io"
	"log"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	argoapp "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestInstallHandler_InstallMDAIHub(t *testing.T) {
	t.Parallel()

	defaultNamespace := "default"

	testCases := []struct {
		description         string
		setupInstallHandler func() *InstallHandler
		validateResult      func(response *connect.Response[emptypb.Empty], err error)
	}{
		{
			description: "unknown error getting integration",
			setupInstallHandler: func() *InstallHandler {
				testConfig := &config.Configuration{
					CurrentNamespace: defaultNamespace,
					Env:              config.Dev,
				}

				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(nil, assert.AnError).
					Once()

				return NewInstallHandler(testConfig, nil, mockArgoIntegration)
			},
			validateResult: func(response *connect.Response[emptypb.Empty], err error) {
				require.Error(t, err)
				require.Nil(t, response)

				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				require.Equal(t, connect.CodeInternal, connectErr.Code())
			},
		},
		{
			description: "integration not found",
			setupInstallHandler: func() *InstallHandler {
				testConfig := &config.Configuration{
					CurrentNamespace: defaultNamespace,
					Env:              config.Dev,
				}

				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(nil, nil).
					Once()

				return NewInstallHandler(testConfig, nil, mockArgoIntegration)
			},
			validateResult: func(response *connect.Response[emptypb.Empty], err error) {
				require.Error(t, err)
				require.Nil(t, response)

				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				require.Equal(t, connect.CodeNotFound, connectErr.Code())
			},
		},
		{
			description: "unknown error installing cert-manager app",
			setupInstallHandler: func() *InstallHandler {
				testConfig := &config.Configuration{
					CurrentNamespace: defaultNamespace,
					Env:              config.Dev,
				}

				mockArgoClient := argocdmock.NewMockAPIClient(t)
				// cert manager app create
				mockArgoClient.EXPECT().
					PushArgoApp(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
						return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
					}), mock.MatchedBy(func(theApp argoapp.Application) bool {
						return theApp.Name == "cert-manager"
					})).
					Return(assert.AnError).
					Once()

				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(&integration.ArgoCDIntegrationData{
						APIUrl:       "http://argocd.com",
						AccountToken: "abc123",
					}, nil).
					Once()

				return NewInstallHandler(testConfig, mockArgoClient, mockArgoIntegration)
			},
			validateResult: func(response *connect.Response[emptypb.Empty], err error) {
				require.Error(t, err)
				require.Nil(t, response)

				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				require.Equal(t, connect.CodeInternal, connectErr.Code())
			},
		},
		{
			description: "unknown error installing mdai app",
			setupInstallHandler: func() *InstallHandler {
				testConfig := &config.Configuration{
					CurrentNamespace: defaultNamespace,
					Env:              config.Dev,
				}

				mockArgoClient := argocdmock.NewMockAPIClient(t)
				// cert manager app create
				mockArgoClient.EXPECT().
					PushArgoApp(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
						return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
					}), mock.MatchedBy(func(theApp argoapp.Application) bool {
						return theApp.Name == "cert-manager"
					})).
					Return(nil).
					Once()
				// mdai app create
				mockArgoClient.EXPECT().
					PushArgoApp(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
						return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
					}), mock.MatchedBy(func(theApp argoapp.Application) bool {
						return theApp.Name == "mdai"
					})).
					Return(assert.AnError).
					Once()

				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(&integration.ArgoCDIntegrationData{
						APIUrl:       "http://argocd.com",
						AccountToken: "abc123",
					}, nil).
					Once()

				return NewInstallHandler(testConfig, mockArgoClient, mockArgoIntegration)
			},
			validateResult: func(response *connect.Response[emptypb.Empty], err error) {
				require.Error(t, err)
				require.Nil(t, response)

				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				require.Equal(t, connect.CodeInternal, connectErr.Code())
			},
		},
		{
			description: "happy path",
			setupInstallHandler: func() *InstallHandler {
				testConfig := &config.Configuration{
					CurrentNamespace: defaultNamespace,
					Env:              config.Dev,
				}

				mockArgoClient := argocdmock.NewMockAPIClient(t)
				// cert manager app create
				mockArgoClient.EXPECT().
					PushArgoApp(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
						return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
					}), mock.MatchedBy(func(theApp argoapp.Application) bool {
						return theApp.Name == "cert-manager"
					})).
					Return(nil).
					Once()
				// mdai app create
				mockArgoClient.EXPECT().
					PushArgoApp(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
						return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
					}), mock.MatchedBy(func(theApp argoapp.Application) bool {
						return theApp.Name == "mdai"
					})).
					Return(nil).
					Once()

				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(&integration.ArgoCDIntegrationData{
						APIUrl:       "http://argocd.com",
						AccountToken: "abc123",
					}, nil).
					Once()

				return NewInstallHandler(testConfig, mockArgoClient, mockArgoIntegration)
			},
			validateResult: func(response *connect.Response[emptypb.Empty], err error) {
				require.NoError(t, err)
				require.NotNil(t, response)
				require.Equal(t, &connect.Response[emptypb.Empty]{}, response)
			},
		},
	}

	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			handler := testCase.setupInstallHandler()

			response, err := handler.InstallMDAIHub(
				t.Context(),
				connect.NewRequest(&octantv1alpha.InstallMDAIHubRequest{
					Namespace:      "mdai",
					ConnectionName: "coolConnection",
					MdaiVersion:    "0.9.0-dev",
				}),
			)

			testCase.validateResult(response, err)
		})
	}
}

func TestInstallHandler_GetInstallStatus(t *testing.T) {
	t.Parallel()

	defaultNamespace := "default"
	theConfig := &config.Configuration{
		CurrentNamespace: defaultNamespace,
		Env:              config.Dev,
		Install: config.Install{
			MdaiInstallTimeout:               10,
			MdaiInstallPollingIntervalMillis: 200, // .2 seconds
		},
	}
	resourceDetails := []*octantv1alpha.ResourceDetails{
		{
			Name:    "mdai-event-hub",
			Message: "we good",
		},
	}
	argoIntegrationData := &integration.ArgoCDIntegrationData{
		APIUrl:       "http://argocd.com",
		AccountToken: "abc123",
	}
	installServiceMethods := octantv1alpha.File_octant_v1alpha_install_service_proto.
		Services().
		ByName("InstallService").
		Methods()

	testCases := []struct {
		description         string
		setupInstallHandler func() *InstallHandler
		validateResult      func(response *connect.ServerStreamForClient[octantv1alpha.GetInstallStatusResponse], err error)
	}{
		{
			description: "error getting argo integration",
			setupInstallHandler: func() *InstallHandler {
				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(nil, assert.AnError).
					Once()
				return NewInstallHandler(theConfig, nil, mockArgoIntegration)
			},
			validateResult: func(response *connect.ServerStreamForClient[octantv1alpha.GetInstallStatusResponse], err error) {
				require.NotNil(t, response)
				require.NoError(t, err)

				response.Receive()
				var connectErr *connect.Error
				require.ErrorAs(t, response.Err(), &connectErr)
				require.Equal(t, connect.CodeInternal, connectErr.Code())
			},
		},
		{
			description: "argo integration not found",
			setupInstallHandler: func() *InstallHandler {
				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(nil, nil).
					Once()
				return NewInstallHandler(theConfig, nil, mockArgoIntegration)
			},
			validateResult: func(response *connect.ServerStreamForClient[octantv1alpha.GetInstallStatusResponse], err error) {
				require.NotNil(t, response)
				require.NoError(t, err)

				response.Receive()
				var connectErr *connect.Error
				require.ErrorAs(t, response.Err(), &connectErr)
				require.Equal(t, connect.CodeNotFound, connectErr.Code())
			},
		},
		{
			description: "error getting app status",
			setupInstallHandler: func() *InstallHandler {
				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(argoIntegrationData, nil).
					Once()

				mockArgoClient := argocdmock.NewMockAPIClient(t)
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
					return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED, nil, assert.AnError).Once()
				return NewInstallHandler(theConfig, mockArgoClient, mockArgoIntegration)
			},
			validateResult: func(response *connect.ServerStreamForClient[octantv1alpha.GetInstallStatusResponse], err error) {
				require.NotNil(t, response)
				require.NoError(t, err)

				response.Receive()
				var connectErr *connect.Error
				require.ErrorAs(t, response.Err(), &connectErr)
				require.Equal(t, connect.CodeInternal, connectErr.Code())
			},
		},
		{
			description: "happy path - errored install",
			setupInstallHandler: func() *InstallHandler {
				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(argoIntegrationData, nil).
					Once()

				mockArgoClient := argocdmock.NewMockAPIClient(t)
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
					return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR, resourceDetails, nil).Once()
				return NewInstallHandler(theConfig, mockArgoClient, mockArgoIntegration)
			},
			validateResult: func(response *connect.ServerStreamForClient[octantv1alpha.GetInstallStatusResponse], err error) {
				require.NoError(t, err)
				require.NotNil(t, response)

				count := 0
				for response.Receive() {
					getInstallResponse := response.Msg()
					require.NoError(t, response.Err())
					if count > 0 {
						require.Fail(t, "shouldn't have received more than one response")
					}
					require.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR, getInstallResponse.GetInstallStatus())
					count++
				}
				require.NoError(t, response.Err())
			},
		},
		{
			description: "error - timeout reached",
			setupInstallHandler: func() *InstallHandler {
				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(argoIntegrationData, nil).
					Once()

				mockArgoClient := argocdmock.NewMockAPIClient(t)
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
					return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING, resourceDetails, nil).Times(1)

				testConfig := &config.Configuration{
					CurrentNamespace: defaultNamespace,
					Env:              config.Dev,
					Install: config.Install{
						MdaiInstallTimeout:               1,
						MdaiInstallPollingIntervalMillis: 2000, // 2 seconds
					},
				}
				return NewInstallHandler(testConfig, mockArgoClient, mockArgoIntegration)
			},
			validateResult: func(response *connect.ServerStreamForClient[octantv1alpha.GetInstallStatusResponse], err error) {
				require.NotNil(t, response)
				require.NoError(t, err)

				count := 0
				for response.Receive() {
					getInstallResponse := response.Msg()
					require.NoError(t, response.Err())
					switch count {
					case 0: // first time it's still installing
						assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING, getInstallResponse.GetInstallStatus())
					case 1: // second response should be timeout
						assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_TIMEOUT, getInstallResponse.GetInstallStatus())
					}
					count++
				}
				require.NoError(t, response.Err())
			},
		},
		{
			description: "happy path",
			setupInstallHandler: func() *InstallHandler {
				mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
				mockArgoIntegration.EXPECT().
					GetIntegrationByName(mock.Anything, "coolConnection").
					Return(argoIntegrationData, nil).
					Once()

				mockArgoClient := argocdmock.NewMockAPIClient(t)
				// returns installing twice, THEN installed
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
					return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING, resourceDetails, nil).Twice()
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
					return opts.AuthToken == "abc123" && opts.ServerAddr == "http://argocd.com" && opts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED, resourceDetails, nil).Once()
				return NewInstallHandler(theConfig, mockArgoClient, mockArgoIntegration)
			},
			validateResult: func(response *connect.ServerStreamForClient[octantv1alpha.GetInstallStatusResponse], err error) {
				require.NoError(t, err)
				require.NotNil(t, response)

				count := 0
				for response.Receive() {
					getInstallResponse := response.Msg()
					require.NoError(t, response.Err())
					switch count {
					case 0, 1:
						assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING, getInstallResponse.GetInstallStatus())
					case 2:
						assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED, getInstallResponse.GetInstallStatus())
					}
					count++
				}
				require.NoError(t, response.Err())
			},
		},
	}

	for _, tc := range testCases {
		testCase := tc
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			handler := testCase.setupInstallHandler()

			installServiceGetInstallStatusHandler := connect.NewServerStreamHandler(
				octantv1alphaconnect.InstallServiceGetInstallStatusProcedure,
				handler.GetInstallStatus,
				connect.WithSchema(installServiceMethods.ByName("GetInstallStatus")),
			)

			testServer := httptest.NewUnstartedServer(installServiceGetInstallStatusHandler)
			testServer.Config.ErrorLog = log.New(io.Discard, "", 0) //nolint:forbidigo
			testServer.EnableHTTP2 = true
			testServer.StartTLS()
			t.Cleanup(testServer.Close)

			client := octantv1alphaconnect.NewInstallServiceClient(testServer.Client(), testServer.URL, connect.WithSendGzip())

			response, err := client.GetInstallStatus(ctx, connect.NewRequest(&octantv1alpha.GetInstallStatusRequest{
				ConnectionName: "coolConnection",
			}))
			testCase.validateResult(response, err)
		})
	}
}
