package rpchandler

import (
	"testing"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/go-faker/faker/v4/pkg/options"
	"github.com/mydecisive/octant/internal/integration"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestDatadogHandler_GetDatadogIntegrations(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		mockIntegration.EXPECT().
			GetIntegrations(mock.Anything).
			Return(map[string]integration.DataDogIntegrationData{
				"coolIntegration1": {},
				"coolIntegration2": {},
			}, nil)

		target := NewDatadogHandler(nil, mockIntegration)
		actual, err := target.GetDatadogIntegrations(t.Context(), connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"coolIntegration1", "coolIntegration2"}, actual.Msg.GetNames())
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		mockIntegration.EXPECT().
			GetIntegrations(mock.Anything).
			Return(nil, assert.AnError)

		target := NewDatadogHandler(nil, mockIntegration)

		actual, err := target.GetDatadogIntegrations(t.Context(), connect.NewRequest(&emptypb.Empty{}))
		require.Error(t, err)
		assert.Nil(t, actual)

		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		require.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestDatadogHandler_SaveIntegration(t *testing.T) {
	t.Parallel()

	var task *octantv1alpha.SaveDatadogIntegrationRequest
	require.NoError(t, faker.FakeData(&task, options.WithRandomMapAndSliceMaxSize(1)))

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		mockIntegration.EXPECT().
			SetIntegration(mock.Anything, task.GetName(), mock.Anything).
			Return(nil)

		target := NewDatadogHandler(nil, mockIntegration)

		_, err := target.SaveDatadogIntegration(t.Context(), connect.NewRequest(task))
		assert.NoError(t, err)
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		mockIntegration.EXPECT().
			SetIntegration(mock.Anything, task.GetName(), mock.Anything).
			Return(assert.AnError)

		target := NewDatadogHandler(nil, mockIntegration)

		_, err := target.SaveDatadogIntegration(t.Context(), connect.NewRequest(task))
		require.Error(t, err)

		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		require.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestDatadogHandler_GetDatadogIntegrationByName(t *testing.T) {
	t.Parallel()

	ddIntegrationData := &integration.DataDogIntegrationData{
		DDUrl:  "https://datadog.com",
		APIKey: "abc123",
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(ddIntegrationData, nil)

		target := NewDatadogHandler(nil, mockIntegration)
		actual, err := target.GetDatadogIntegrationByName(t.Context(), connect.NewRequest(&octantv1alpha.GetDatadogIntegrationByNameRequest{
			Name: "coolIntegration",
		}))
		require.NoError(t, err)
		assert.Equal(t, "https://datadog.com", actual.Msg.GetUrl())
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(nil, assert.AnError)

		target := NewDatadogHandler(nil, mockIntegration)
		actual, err := target.GetDatadogIntegrationByName(t.Context(), connect.NewRequest(&octantv1alpha.GetDatadogIntegrationByNameRequest{
			Name: "coolIntegration",
		}))
		require.Error(t, err)
		assert.Nil(t, actual)

		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		require.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}
