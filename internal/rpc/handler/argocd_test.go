package rpchandler

import (
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestArgoCDHandler_TestConnection(t *testing.T) {
	t.Parallel()

	defaultNamespace := "default"

	t.Run("error testing connection", func(t *testing.T) {
		t.Parallel()

		testConfig := &config.Configuration{
			Env: config.Dev,
			RPC: config.RPC{
				Port: 1234,
			},
		}
		mockArgoCDClient := argocdmock.NewMockAPIClient(t)
		mockArgoCDClient.EXPECT().TestConnection(mock.Anything, mock.Anything, mock.Anything).Return(false, assert.AnError).Once()

		handler := NewArgoCDHandler(testConfig, mockArgoCDClient, nil, defaultNamespace)

		response, err := handler.TestConnection(
			t.Context(),
			connect.NewRequest(&octantv1alpha.TestConnectionRequest{
				ArgoAccountToken: "abc123",
				ArgoEndpoint:     "https://argocd-server",
			}),
		)
		require.Error(t, err)
		require.Nil(t, response)

		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		require.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		testConfig := &config.Configuration{
			Env: config.Dev,
			RPC: config.RPC{
				Port: 1234,
			},
		}
		mockArgoCDClient := argocdmock.NewMockAPIClient(t)
		mockArgoCDClient.EXPECT().TestConnection(mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()

		handler := NewArgoCDHandler(testConfig, mockArgoCDClient, nil, defaultNamespace)

		response, err := handler.TestConnection(
			t.Context(),
			connect.NewRequest(&octantv1alpha.TestConnectionRequest{
				ArgoAccountToken: "abc123",
				ArgoEndpoint:     "https://argocd-server",
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, &connect.Response[octantv1alpha.TestConnectionResponse]{
			Msg: &octantv1alpha.TestConnectionResponse{
				Success: true,
			},
		}, response)
	})
}

func TestArgoCDHandler_SaveArgoConnection(t *testing.T) {
	t.Parallel()

	defaultNamespace := "default"

	t.Run("error saving argo details", func(t *testing.T) {
		t.Parallel()

		mockArgoCDClient := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoCDClient.EXPECT().
			SetIntegration(mock.Anything, defaultNamespace, "mdai", mock.MatchedBy(func(integrationData any) bool {
				argocdIntegrationData, ok := integrationData.(integration.ArgoCDIntegrationData)
				return ok && argocdIntegrationData.AccountToken == "abc123"
			})).
			Return(assert.AnError).
			Times(1)

		handler := NewArgoCDHandler(nil, nil, mockArgoCDClient, defaultNamespace)
		response, err := handler.SaveArgoConnection(
			t.Context(),
			connect.NewRequest(&octantv1alpha.SaveArgoConnectionRequest{
				ArgoAccountToken: "abc123",
				ArgoEndpoint:     "https://argocd-server",
			}),
		)
		require.Error(t, err)
		require.Nil(t, response)

		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		require.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoCDClient := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoCDClient.EXPECT().
			SetIntegration(mock.Anything, defaultNamespace, "mdai", mock.MatchedBy(func(integrationData any) bool {
				argocdIntegrationData, ok := integrationData.(integration.ArgoCDIntegrationData)
				return ok && argocdIntegrationData.AccountToken == "abc123"
			})).
			Return(nil).
			Times(1)

		handler := NewArgoCDHandler(nil, nil, mockArgoCDClient, defaultNamespace)
		response, err := handler.SaveArgoConnection(
			t.Context(),
			connect.NewRequest(&octantv1alpha.SaveArgoConnectionRequest{
				ArgoAccountToken: "abc123",
				ArgoEndpoint:     "https://argocd-server",
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, &connect.Response[emptypb.Empty]{}, response)
	})
}
