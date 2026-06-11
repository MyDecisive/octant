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
	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection/manifest"
	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	manifestmock "github.com/mydecisive/octant/internal/mock/manifest"
	manifestdatamock "github.com/mydecisive/octant/internal/mock/manifestdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInstallHandler_InstallMDAIHub(t *testing.T) {
	t.Parallel()

	request := octantv1alpha.InstallMDAIHubRequest{
		Namespace:      faker.Word(),
		ConnectionName: faker.Word(),
		MdaiVersion:    faker.Word(),
	}

	inputCertAppData := manifestdata.AppTemplateData{
		Namespace: faker.Word(),
	}
	inputAppAppData := manifestdata.AppTemplateData{
		Version: request.GetMdaiVersion(),
	}

	managerInputMatch := func(input manifest.ManagerInput) bool {
		return input.DeploymentIntegrationName == request.GetConnectionName()
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockMapper := manifestdatamock.NewMockMapper(t)
		mockMapper.EXPECT().AppTemplateData(manifestdata.CERT, "", "", "").Return(inputCertAppData).Once()
		mockMapper.EXPECT().AppTemplateData(manifestdata.MDAI, request.GetMdaiVersion(), "", request.GetNamespace()).Return(inputAppAppData).Once()
		mockManager := manifestmock.NewMockManager(t)
		mockManager.EXPECT().LoadCertManager(mock.Anything, mock.MatchedBy(managerInputMatch), inputCertAppData).Return(nil).Once()
		mockManager.EXPECT().LoadMDAI(mock.Anything, mock.MatchedBy(managerInputMatch), inputAppAppData).Return(nil).Once()

		target := NewInstallHandler(nil, nil, nil, mockMapper, mockManager)
		_, err := target.InstallMDAIHub(t.Context(), connect.NewRequest(&request))
		assert.NoError(t, err)
	})

	t.Run("err cert manager", func(t *testing.T) {
		t.Parallel()

		mockMapper := manifestdatamock.NewMockMapper(t)
		mockMapper.EXPECT().AppTemplateData(manifestdata.CERT, "", "", "").Return(inputCertAppData).Once()
		mockManager := manifestmock.NewMockManager(t)
		mockManager.EXPECT().LoadCertManager(mock.Anything, mock.MatchedBy(managerInputMatch), inputCertAppData).Return(assert.AnError).Once()

		target := NewInstallHandler(nil, nil, nil, mockMapper, mockManager)
		_, err := target.InstallMDAIHub(t.Context(), connect.NewRequest(&request))
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
		assert.Contains(t, connectErr.Message(), "cert")
	})

	t.Run("err mdai", func(t *testing.T) {
		t.Parallel()

		mockMapper := manifestdatamock.NewMockMapper(t)
		mockMapper.EXPECT().AppTemplateData(manifestdata.CERT, "", "", "").Return(inputCertAppData).Once()
		mockMapper.EXPECT().AppTemplateData(manifestdata.MDAI, request.GetMdaiVersion(), "", request.GetNamespace()).Return(inputAppAppData).Once()
		mockManager := manifestmock.NewMockManager(t)
		mockManager.EXPECT().LoadCertManager(mock.Anything, mock.MatchedBy(managerInputMatch), inputCertAppData).Return(nil).Once()
		mockManager.EXPECT().LoadMDAI(mock.Anything, mock.MatchedBy(managerInputMatch), inputAppAppData).Return(assert.AnError).Once()

		target := NewInstallHandler(nil, nil, nil, mockMapper, mockManager)
		_, err := target.InstallMDAIHub(t.Context(), connect.NewRequest(&request))
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
		assert.Contains(t, connectErr.Message(), "MDAI")
	})
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
				return NewInstallHandler(theConfig, nil, mockArgoIntegration, nil, nil)
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
				return NewInstallHandler(theConfig, nil, mockArgoIntegration, nil, nil)
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
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.MatchedBy(func(in argocd.Input) bool {
					return in.ClientOpts.AuthToken == "abc123" &&
						in.ClientOpts.ServerAddr == "http://argocd.com" &&
						in.ClientOpts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED, nil, assert.AnError).Once()
				return NewInstallHandler(theConfig, mockArgoClient, mockArgoIntegration, nil, nil)
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
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.MatchedBy(func(in argocd.Input) bool {
					return in.ClientOpts.AuthToken == "abc123" &&
						in.ClientOpts.ServerAddr == "http://argocd.com" &&
						in.ClientOpts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR, resourceDetails, nil).Times(3)

				testConfig := &config.Configuration{
					CurrentNamespace: defaultNamespace,
					Env:              config.Dev,
					Install: config.Install{
						MdaiInstallTimeout:               1,
						MdaiInstallPollingIntervalMillis: 400, // 0.4 seconds
					},
				}
				return NewInstallHandler(testConfig, mockArgoClient, mockArgoIntegration, nil, nil)
			},
			validateResult: func(response *connect.ServerStreamForClient[octantv1alpha.GetInstallStatusResponse], err error) {
				require.NoError(t, err)
				require.NotNil(t, response)

				count := 0
				for response.Receive() {
					getInstallResponse := response.Msg()
					require.NoError(t, response.Err())
					switch count {
					case 0, 1, 2:
						assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_ERROR, getInstallResponse.GetInstallStatus())
					case 3:
						assert.Equal(t, octantv1alpha.InstallStatus_INSTALL_STATUS_TIMEOUT, getInstallResponse.GetInstallStatus())
					}
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
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.MatchedBy(func(in argocd.Input) bool {
					return in.ClientOpts.AuthToken == "abc123" &&
						in.ClientOpts.ServerAddr == "http://argocd.com" &&
						in.ClientOpts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING, resourceDetails, nil).Times(1)

				testConfig := &config.Configuration{
					CurrentNamespace: defaultNamespace,
					Env:              config.Dev,
					Install: config.Install{
						MdaiInstallTimeout:               1,
						MdaiInstallPollingIntervalMillis: 2000, // 2 seconds
					},
				}
				return NewInstallHandler(testConfig, mockArgoClient, mockArgoIntegration, nil, nil)
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
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.MatchedBy(func(in argocd.Input) bool {
					return in.ClientOpts.AuthToken == "abc123" &&
						in.ClientOpts.ServerAddr == "http://argocd.com" &&
						in.ClientOpts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLING, resourceDetails, nil).Twice()
				mockArgoClient.EXPECT().GetAppStatus(mock.Anything, mock.MatchedBy(func(in argocd.Input) bool {
					return in.ClientOpts.AuthToken == "abc123" &&
						in.ClientOpts.ServerAddr == "http://argocd.com" &&
						in.ClientOpts.Insecure
				})).Return(octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED, resourceDetails, nil).Once()
				return NewInstallHandler(theConfig, mockArgoClient, mockArgoIntegration, nil, nil)
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
