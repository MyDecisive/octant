package argocd

import (
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applicationmock "github.com/mydecisive/octant/internal/mock/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"testing"
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

			lis, err := net.Listen("tcp", "127.0.0.1:0")
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
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Create(mock.Anything, mock.MatchedBy(func(req *application.ApplicationCreateRequest) bool {
					return req.GetUpsert() == true && req.GetApplication() != nil
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
				mockAppServer := applicationmock.NewMockApplicationServiceServer(t)
				mockAppServer.EXPECT().Create(mock.Anything, mock.MatchedBy(func(req *application.ApplicationCreateRequest) bool {
					return req.GetUpsert() == true && req.GetApplication() != nil
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

			lis, err := net.Listen("tcp", "127.0.0.1:0")
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
