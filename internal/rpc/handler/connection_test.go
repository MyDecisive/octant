package rpchandler

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/connection"
	connectionmock "github.com/mydecisive/octant/internal/mock/connection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
			GetConnectionValidatorRuns(mock.Anything, "test-ns", "test-conn").
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
			PutConnectionValidatorRun(mock.Anything, "test-ns", "test-conn").
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
			DeleteConnectionValidator(mock.Anything, "test-ns", "test-conn").
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
