package rpchandler

import (
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/octant/internal/config"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	"github.com/stretchr/testify/mock"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestArgoCDHandler_TestConnection(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		testConfig := &config.Configuration{
			Env: config.Dev,
			RPC: config.RPC{
				Port: 1234,
			},
		}
		mockArgoCDClient := argocdmock.NewMockAPIClient(t)
		mockArgoCDClient.EXPECT().TestConnection(mock.Anything, mock.Anything, mock.Anything).Return(true).Once()

		handler := NewArgoCDHandler(testConfig, mockArgoCDClient)

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

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		handler := NewArgoCDHandler(nil, nil)
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
