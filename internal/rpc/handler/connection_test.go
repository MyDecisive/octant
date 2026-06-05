package rpchandler

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	connectionmock "github.com/mydecisive/octant/internal/mock/connection"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestConnectionHandler_GenerateManifests(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		expected := faker.Word()
		mockCompressor := connectionmock.NewMockManifestCompressor(t)
		mockCompressor.EXPECT().CreateCompressed(mock.Anything, mock.Anything).Return(bytes.NewBufferString(expected), nil)

		target := NewConnectionHandler(nil, nil, mockCompressor)
		_, handler := octantv1alphaconnect.NewConnectionServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := octantv1alphaconnect.NewConnectionServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.GenerateManifests(t.Context(), connect.NewRequest(&octantv1alpha.GenerateManifestsRequest{
			MdaiVersion: "0.9.0-dev",
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      faker.Word(),
				ConnectionName: faker.Word(),
			},
			Format:         octantv1alpha.ManifestOutFormat_MANIFEST_OUT_FORMAT_YAML,
			DeploymentType: octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_MANIFEST,
			TelemetryTypes: []octantv1alpha.MLTType{octantv1alpha.MLTType_MLT_TYPE_LOG},
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)
		require.True(t, stream.Receive())
		assert.Equal(t, []byte(expected), bytes.Trim(stream.Msg().GetData(), "\x00"))
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockCompressor := connectionmock.NewMockManifestCompressor(t)
		mockCompressor.EXPECT().CreateCompressed(mock.Anything, mock.Anything).Return(nil, assert.AnError)

		target := NewConnectionHandler(nil, nil, mockCompressor)
		_, handler := octantv1alphaconnect.NewConnectionServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := octantv1alphaconnect.NewConnectionServiceClient(testServer.Client(), testServer.URL)
		stream, _ := client.GenerateManifests(t.Context(), connect.NewRequest(&octantv1alpha.GenerateManifestsRequest{
			MdaiVersion: "0.9.0-dev",
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      faker.Word(),
				ConnectionName: faker.Word(),
			},
			Format:         octantv1alpha.ManifestOutFormat_MANIFEST_OUT_FORMAT_YAML,
			DeploymentType: octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_MANIFEST,
			TelemetryTypes: []octantv1alpha.MLTType{octantv1alpha.MLTType_MLT_TYPE_LOG},
		}))
		stream.Receive()
		assert.Error(t, stream.Err())
	})
}

func TestConnectionHandler_ValidatorEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("GetConnectionValidatorRunIds - success", func(t *testing.T) {
		t.Parallel()
		expectedRuns := []string{"run-1", "run-2"}

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().
			GetConnectionValidatorRuns(mock.Anything, mock.MatchedBy(func(input connection.ConnectionCRUDInput) bool {
				return input.Namespace == "test-ns" && input.ConnectionName == "test-conn"
			})).
			Return(expectedRuns, nil)

		target := NewConnectionHandler(nil, mockConn, nil)
		resp, err := target.GetConnectionValidatorRunIds(t.Context(), connect.NewRequest(&octantv1alpha.GetConnectionValidatorRunIdsRequest{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      "test-ns",
				ConnectionName: "test-conn",
			},
		}))

		require.NoError(t, err)
		assert.Equal(t, expectedRuns, resp.Msg.GetValidatorRunIds())
	})

	t.Run("CreateConnectionValidatorRun - success", func(t *testing.T) {
		t.Parallel()
		expectedRunID := "new-run-id-123"

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().
			PutConnectionValidatorRun(mock.Anything, mock.MatchedBy(func(input connection.ConnectionCRUDInput) bool {
				return input.Namespace == "test-ns" && input.ConnectionName == "test-conn"
			})).
			Return(expectedRunID, nil)

		target := NewConnectionHandler(nil, mockConn, nil)
		resp, err := target.CreateConnectionValidatorRun(t.Context(), connect.NewRequest(&octantv1alpha.CreateConnectionValidatorRunRequest{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      "test-ns",
				ConnectionName: "test-conn",
			},
		}))

		require.NoError(t, err)
		assert.Equal(t, expectedRunID, resp.Msg.GetValidatorRunId())
	})

	t.Run("DeleteConnectionValidator - success", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().
			DeleteConnectionValidator(mock.Anything, mock.MatchedBy(func(input connection.ConnectionCRUDInput) bool {
				return input.Namespace == "test-ns" && input.ConnectionName == "test-conn"
			})).
			Return(nil)

		target := NewConnectionHandler(nil, mockConn, nil)
		_, err := target.DeleteConnectionValidator(t.Context(), connect.NewRequest(&octantv1alpha.DeleteConnectionValidatorRequest{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      "test-ns",
				ConnectionName: "test-conn",
			},
		}))

		require.NoError(t, err)
	})
}

func TestConnectionHandler_GetConnectionStatus(t *testing.T) {
	t.Parallel()

	mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
	expectedResponse := &octantv1alpha.GetConnectionStatusResponse{
		ReceivingData: true,
		SendingData:   true,
		DataIntegrity: true,
	}

	mockConn.EXPECT().
		GetConnectionStatus(mock.Anything, mock.MatchedBy(func(input connection.ConnectionCRUDInput) bool {
			return input.Namespace == "test-ns" && input.ConnectionName == "test-conn"
		}), "test-run").
		Return(expectedResponse, nil)

	target := NewConnectionHandler(nil, mockConn, nil)
	resp, err := target.GetConnectionStatus(t.Context(), connect.NewRequest(&octantv1alpha.GetConnectionStatusRequest{
		Scope: &octantv1alpha.ConnectionScope{
			Namespace:      "test-ns",
			ConnectionName: "test-conn",
		},
		ValidatorRunId: "test-run",
	}))

	require.NoError(t, err)
	assert.Equal(t, expectedResponse.GetReceivingData(), resp.Msg.GetReceivingData())
}

func TestConnectionHandler_GetConnections(t *testing.T) {
	t.Parallel()

	mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
	expectedConns := []string{"conn-a", "conn-b"}

	mockConn.EXPECT().
		GetConnections(mock.Anything, mock.Anything).
		Return(expectedConns, nil)

	target := NewConnectionHandler(nil, mockConn, nil)
	resp, err := target.GetConnections(t.Context(), connect.NewRequest(&emptypb.Empty{}))

	require.NoError(t, err)
	assert.ElementsMatch(t, expectedConns, resp.Msg.GetConnectionNames())
}

func TestConnectionHandler_GetConnection(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockData := &connection.OctantConnectionData{
			SourceType:     "octant",
			TelemetryTypes: []telemetry.MLT{telemetry.Logs, telemetry.Metrics},
			Deployment: &connection.Deployment{
				Type:            connection.ArgoSideloadDeploymentType,
				IntegrationName: "cool-integration",
			},
			Destinations: []connection.OctantConnectionDestination{
				{DestinationType: "datadog", IntegrationName: "cool-integration"},
			},
			MdaiNamespace: "mdai",
		}

		mockConn.EXPECT().
			GetConnectionByName(mock.Anything, mock.MatchedBy(func(input connection.ConnectionCRUDInput) bool {
				return input.ConnectionName == "test-conn"
			})).
			Return(mockData, nil)

		target := NewConnectionHandler(nil, mockConn, nil)
		resp, err := target.GetConnection(t.Context(), connect.NewRequest(&octantv1alpha.GetConnectionRequest{
			ConnectionName: "test-conn",
		}))

		connectionData := resp.Msg.GetConnectionData()
		require.NoError(t, err)
		assert.Contains(t, connectionData.GetTelemetryTypes(), octantv1alpha.MLTType_MLT_TYPE_LOG)
		assert.Equal(t, octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_SIDELOAD, connectionData.GetDeployment().GetType())
		assert.Len(t, connectionData.GetDestinations(), 1)
		assert.Equal(t, octantv1alpha.IntegrationType_INTEGRATION_TYPE_DATADOG, connectionData.GetDestinations()[0].GetType())
		assert.Equal(t, "mdai", connectionData.GetScope().GetNamespace())
		assert.Equal(t, "cool-integration", connectionData.GetScope().GetConnectionName())
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().
			GetConnectionByName(mock.Anything, mock.MatchedBy(func(input connection.ConnectionCRUDInput) bool {
				return input.ConnectionName == "missing-conn"
			})).
			Return(nil, nil) // returns nil, nil when not found

		target := NewConnectionHandler(nil, mockConn, nil)
		_, err := target.GetConnection(t.Context(), connect.NewRequest(&octantv1alpha.GetConnectionRequest{
			ConnectionName: "missing-conn",
		}))

		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})
}

func TestConnectionHandler_CreateConnection(t *testing.T) {
	t.Parallel()

	conf := &config.Configuration{
		Budget: config.Budget{
			FilterSettingUpdateTimeout: 1,
		},
	}

	input := &octantv1alpha.CreateConnectionRequest{
		ConnectionData: &octantv1alpha.ConnectionData{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      "test-ns",
				ConnectionName: "test-conn",
			},
			TelemetryTypes: []octantv1alpha.MLTType{octantv1alpha.MLTType_MLT_TYPE_LOG},
			Deployment: &octantv1alpha.Deployment{
				Type:            octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_SIDELOAD,
				IntegrationName: "test-argo",
			},
			Destinations: []*octantv1alpha.TelemetryDestination{
				{
					Type:            octantv1alpha.IntegrationType_INTEGRATION_TYPE_DATADOG,
					IntegrationName: "test-dd",
				},
			},
		},
	}
	inputScope := input.GetConnectionData().GetScope()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().
			SaveConnection(mock.Anything, mock.MatchedBy(func(data connection.OctantConnectionData) bool {
				return data.Deployment.Type == connection.ArgoSideloadDeploymentType &&
					len(data.Destinations) == 1 &&
					len(data.TelemetryTypes) == 1
			}), mock.MatchedBy(func(actual connection.ConnectionCRUDInput) bool {
				return actual.Namespace == inputScope.GetNamespace() &&
					actual.ConnectionName == inputScope.GetConnectionName()
			})).
			Return(nil)

		target := NewConnectionHandler(conf, mockConn, nil)
		_, err := target.CreateConnection(t.Context(), connect.NewRequest(input))

		require.NoError(t, err)
	})
}

func TestConnectionHandler_DeleteConnection(t *testing.T) {
	t.Parallel()

	mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
	mockConn.EXPECT().
		DeleteConnection(mock.Anything, mock.MatchedBy(func(input connection.ConnectionCRUDInput) bool {
			return input.ConnectionName == "test-conn"
		})).
		Return(nil)

	target := NewConnectionHandler(nil, mockConn, nil)
	_, err := target.DeleteConnection(t.Context(), connect.NewRequest(&octantv1alpha.DeleteConnectionRequest{
		ConnectionName: "test-conn",
	}))

	require.NoError(t, err)
}
