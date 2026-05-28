package rpchandler

import (
	"testing"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestArgoCDHandler_TestConnection(t *testing.T) {
	t.Parallel()

	defaultNamespace := "default"

	t.Run("error testing connection", func(t *testing.T) {
		t.Parallel()

		testConfig := &config.Configuration{
			Env:              config.Dev,
			Port:             1234,
			CurrentNamespace: defaultNamespace,
		}
		mockArgoCDClient := argocdmock.NewMockAPIClient(t)
		mockArgoCDClient.EXPECT().
			TestConnection(mock.Anything, mock.Anything, mock.Anything).
			Return(false, assert.AnError).
			Once()

		handler := NewArgoCDHandler(testConfig, mockArgoCDClient, nil)

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
			Env:              config.Dev,
			Port:             1234,
			CurrentNamespace: defaultNamespace,
		}
		mockArgoCDClient := argocdmock.NewMockAPIClient(t)
		mockArgoCDClient.EXPECT().TestConnection(mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()

		handler := NewArgoCDHandler(testConfig, mockArgoCDClient, nil)

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
			SetIntegration(mock.Anything, "coolConnection", mock.MatchedBy(func(integrationData any) bool {
				argocdIntegrationData, ok := integrationData.(integration.ArgoCDIntegrationData)
				return ok && argocdIntegrationData.AccountToken == "abc123"
			})).
			Return(assert.AnError).
			Times(1)

		testConfig := &config.Configuration{
			CurrentNamespace: defaultNamespace,
		}
		handler := NewArgoCDHandler(testConfig, nil, mockArgoCDClient)
		response, err := handler.SaveArgoConnection(
			t.Context(),
			connect.NewRequest(&octantv1alpha.SaveArgoConnectionRequest{
				ArgoAccountToken: "abc123",
				ArgoEndpoint:     "https://argocd-server",
				Name:             "coolConnection",
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
			SetIntegration(mock.Anything, "coolConnection", mock.MatchedBy(func(integrationData any) bool {
				argocdIntegrationData, ok := integrationData.(integration.ArgoCDIntegrationData)
				return ok && argocdIntegrationData.AccountToken == "abc123"
			})).
			Return(nil).
			Times(1)

		testConfig := &config.Configuration{
			CurrentNamespace: defaultNamespace,
		}
		handler := NewArgoCDHandler(testConfig, nil, mockArgoCDClient)
		response, err := handler.SaveArgoConnection(
			t.Context(),
			connect.NewRequest(&octantv1alpha.SaveArgoConnectionRequest{
				ArgoAccountToken: "abc123",
				ArgoEndpoint:     "https://argocd-server",
				Name:             "coolConnection",
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, &connect.Response[emptypb.Empty]{}, response)
	})
}

func TestArgoCDHandler_GetArgoIntegrations(t *testing.T) {
	t.Parallel()

	integrations := map[string]integration.ArgoCDIntegrationData{
		"coolConnection1": {
			AccountToken: "abc123",
			APIUrl:       "https://argocd-server",
		},
		"coolConnection2": {
			AccountToken: "abc123",
			APIUrl:       "https://argocd-server",
		},
	}

	t.Run("error retrieving argo integrations", func(t *testing.T) {
		t.Parallel()

		mockArgoCDIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoCDIntegration.EXPECT().
			GetIntegrations(mock.Anything).
			Return(nil, assert.AnError).
			Times(1)

		handler := NewArgoCDHandler(nil, nil, mockArgoCDIntegration)
		response, err := handler.GetArgoIntegrations(t.Context(), connect.NewRequest(&emptypb.Empty{}))
		require.Error(t, err)
		require.Nil(t, response)

		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		require.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoCDIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoCDIntegration.EXPECT().
			GetIntegrations(mock.Anything).
			Return(integrations, nil).
			Times(1)

		handler := NewArgoCDHandler(nil, nil, mockArgoCDIntegration)
		response, err := handler.GetArgoIntegrations(t.Context(), connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.NotNil(t, response)
		require.ElementsMatch(t, []string{"coolConnection1", "coolConnection2"}, response.Msg.GetNames())
	})
}

func TestArgoCDHandler_GetArgoIntegrationByName(t *testing.T) {
	t.Parallel()

	theIntegration := &integration.ArgoCDIntegrationData{
		AccountToken: "abc123",
		APIUrl:       "https://argocd-server",
	}

	t.Run("error retrieving argo integration by name", func(t *testing.T) {
		t.Parallel()

		mockArgoCDIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoCDIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(nil, assert.AnError).
			Times(1)

		handler := NewArgoCDHandler(nil, nil, mockArgoCDIntegration)
		response, err := handler.GetArgoIntegrationByName(t.Context(), connect.NewRequest(&octantv1alpha.GetArgoIntegrationByNameRequest{
			Name: "coolIntegration",
		}))
		require.Error(t, err)
		require.Nil(t, response)

		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		require.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoCDIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoCDIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(theIntegration, nil).
			Times(1)

		handler := NewArgoCDHandler(nil, nil, mockArgoCDIntegration)
		response, err := handler.GetArgoIntegrationByName(t.Context(), connect.NewRequest(&octantv1alpha.GetArgoIntegrationByNameRequest{
			Name: "coolIntegration",
		}))
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, "https://argocd-server", response.Msg.GetArgoEndpoint())
	})
}
