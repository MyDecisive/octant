package rpchandler

import (
	"testing"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/go-faker/faker/v4/pkg/options"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestDatadogHandler_GetIntegrations(t *testing.T) {
	t.Parallel()

	configuration := &config.Configuration{
		CurrentNamespace: faker.Word(),
	}
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		expected := faker.Word()
		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		mockIntegration.EXPECT().
			GetIntegrations(testifymock.Anything, configuration.CurrentNamespace).
			Return(map[string]integration.DataDogIntegrationData{
				expected: {},
			}, nil)

		target := NewDatadogHandler(configuration, mockIntegration)

		actual, err := target.GetDatadogIntegrations(t.Context(), connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)

		assert.Contains(t, actual.Msg.GetNames(), expected)
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		mockIntegration.EXPECT().
			GetIntegrations(testifymock.Anything, configuration.CurrentNamespace).
			Return(nil, assert.AnError)

		target := NewDatadogHandler(configuration, mockIntegration)

		actual, err := target.GetDatadogIntegrations(t.Context(), connect.NewRequest(&emptypb.Empty{}))
		require.Error(t, err)
		assert.Nil(t, actual)
	})
}

func TestDatadogHandler_SaveIntegration(t *testing.T) {
	t.Parallel()

	configuration := &config.Configuration{
		CurrentNamespace: faker.Word(),
	}

	var task *octantv1alpha.SaveDatadogIntegrationRequest
	require.NoError(t, faker.FakeData(&task, options.WithRandomMapAndSliceMaxSize(1)))

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		mockIntegration.EXPECT().
			SetIntegration(testifymock.Anything, configuration.CurrentNamespace, task.GetName(), testifymock.Anything).
			Return(nil)

		target := NewDatadogHandler(configuration, mockIntegration)

		_, err := target.SaveDatadogIntegration(t.Context(), connect.NewRequest(task))
		assert.NoError(t, err)
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		mockIntegration.EXPECT().
			SetIntegration(testifymock.Anything, configuration.CurrentNamespace, task.GetName(), testifymock.Anything).
			Return(assert.AnError)

		target := NewDatadogHandler(configuration, mockIntegration)

		_, err := target.SaveDatadogIntegration(t.Context(), connect.NewRequest(task))
		assert.Error(t, err)
	})
}
