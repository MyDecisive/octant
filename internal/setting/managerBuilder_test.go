package setting

import (
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	connectionmock "github.com/mydecisive/octant/internal/mock/connection"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSettingManagerBuilder_Build(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	connectionName := faker.Word()
	logger := zaptest.NewLogger(t)

	c := &config.Configuration{}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		expectedConn := new(connection.OctantConnectionData)
		expectedDatadog := new(integration.DataDogIntegrationData)
		expectedArgo := new(integration.ArgoCDIntegrationData)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(mock.Anything, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == namespace && in.ConnectionName == connectionName
		})).Return(expectedConn, nil).Once()
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadog.EXPECT().GetIntegrationByName(mock.Anything, connectionName).Return(expectedDatadog, nil).Once()
		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, connectionName).Return(expectedArgo, nil).Once()
		mockArgoClient := argocdmock.NewMockAPIClient(t)

		target := NewSettingManagerBuilder(c, mockConn, mockDatadog, mockArgoClient, mockArgo)
		actual, err := target.Build(t.Context(), namespace, connectionName, logger)
		require.NoError(t, err)

		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Equal(t, mockArgoClient, actualManager.argoClient)
		assert.Equal(t, mockConn, actualManager.connectionService)
		assert.Equal(t, mockDatadog, actualManager.datadogService)
		assert.Equal(t, c, actualManager.configuration)

		assert.Equal(t, expectedArgo, actualManager.argo)
		assert.Equal(t, expectedDatadog, actualManager.datadog)
		assert.Equal(t, expectedConn, actualManager.connection)

		assert.Equal(t, connectionName, actualManager.connectionName)
		assert.Equal(t, namespace, actualManager.namespace)
		assert.Equal(t, logger, actualManager.logger)

		assert.False(t, actualManager.shouldUpdateConnection)
		assert.False(t, actualManager.shouldUpdateDatadog)

		assert.NotEmpty(t, target.inProgress)
	})

	t.Run("err already in progress", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)

		target := NewSettingManagerBuilder(c, mockConn, mockDatadog, mockArgoClient, mockArgo)
		target.inProgress[connectionName] = faker.Word()
		actual, err := target.Build(t.Context(), namespace, connectionName, logger)
		assert.NotEmpty(t, target.inProgress)
		assert.Nil(t, actual)
		assert.ErrorIs(t, err, ErrStillUpdating)
	})

	t.Run("err conn", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(mock.Anything, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == namespace && in.ConnectionName == connectionName
		})).Return(nil, assert.AnError).Once()
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)

		target := NewSettingManagerBuilder(c, mockConn, mockDatadog, mockArgoClient, mockArgo)
		actual, err := target.Build(t.Context(), namespace, connectionName, logger)
		assert.Empty(t, target.inProgress)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})

	t.Run("err empty conn", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(mock.Anything, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == namespace && in.ConnectionName == connectionName
		})).Return(nil, nil).Once()
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)

		target := NewSettingManagerBuilder(c, mockConn, mockDatadog, mockArgoClient, mockArgo)
		actual, err := target.Build(t.Context(), namespace, connectionName, logger)
		assert.Empty(t, target.inProgress)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})

	t.Run("err datadog", func(t *testing.T) {
		t.Parallel()

		expectedConn := new(connection.OctantConnectionData)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(mock.Anything, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == namespace && in.ConnectionName == connectionName
		})).Return(expectedConn, nil).Once()
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadog.EXPECT().GetIntegrationByName(mock.Anything, connectionName).Return(nil, assert.AnError).Once()
		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)

		target := NewSettingManagerBuilder(c, mockConn, mockDatadog, mockArgoClient, mockArgo)
		actual, err := target.Build(t.Context(), namespace, connectionName, logger)
		assert.Empty(t, target.inProgress)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})

	t.Run("err empty datadog", func(t *testing.T) {
		t.Parallel()

		expectedConn := new(connection.OctantConnectionData)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(mock.Anything, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == namespace && in.ConnectionName == connectionName
		})).Return(expectedConn, nil).Once()
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadog.EXPECT().GetIntegrationByName(mock.Anything, connectionName).Return(nil, nil).Once()
		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)

		target := NewSettingManagerBuilder(c, mockConn, mockDatadog, mockArgoClient, mockArgo)
		actual, err := target.Build(t.Context(), namespace, connectionName, logger)
		assert.Empty(t, target.inProgress)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})

	t.Run("err argo", func(t *testing.T) {
		t.Parallel()

		expectedConn := new(connection.OctantConnectionData)
		expectedDatadog := new(integration.DataDogIntegrationData)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(mock.Anything, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == namespace && in.ConnectionName == connectionName
		})).Return(expectedConn, nil).Once()
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadog.EXPECT().GetIntegrationByName(mock.Anything, connectionName).Return(expectedDatadog, nil).Once()
		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, connectionName).Return(nil, assert.AnError).Once()
		mockArgoClient := argocdmock.NewMockAPIClient(t)

		target := NewSettingManagerBuilder(c, mockConn, mockDatadog, mockArgoClient, mockArgo)
		actual, err := target.Build(t.Context(), namespace, connectionName, logger)
		assert.Empty(t, target.inProgress)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})

	t.Run("err empty argo", func(t *testing.T) {
		t.Parallel()

		expectedConn := new(connection.OctantConnectionData)
		expectedDatadog := new(integration.DataDogIntegrationData)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(mock.Anything, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == namespace && in.ConnectionName == connectionName
		})).Return(expectedConn, nil).Once()
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadog.EXPECT().GetIntegrationByName(mock.Anything, connectionName).Return(expectedDatadog, nil).Once()
		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, connectionName).Return(nil, nil).Once()
		mockArgoClient := argocdmock.NewMockAPIClient(t)

		target := NewSettingManagerBuilder(c, mockConn, mockDatadog, mockArgoClient, mockArgo)
		actual, err := target.Build(t.Context(), namespace, connectionName, logger)
		assert.Empty(t, target.inProgress)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})
}

func TestSettingManagerBuilder_Release(t *testing.T) {
	t.Parallel()

	connectionName := faker.Word()

	c := &config.Configuration{}

	t.Run("removed", func(t *testing.T) {
		t.Parallel()

		target := NewSettingManagerBuilder(c, nil, nil, nil, nil)
		id := faker.Word()
		target.inProgress[connectionName] = id

		target.Release(connectionName, id)

		assert.Empty(t, target.inProgress)
	})

	t.Run("wrong id", func(t *testing.T) {
		t.Parallel()

		target := NewSettingManagerBuilder(c, nil, nil, nil, nil)
		id := faker.Word()
		target.inProgress[connectionName] = faker.UUIDDigit()

		target.Release(connectionName, id)

		assert.Contains(t, target.inProgress, connectionName)
	})

	t.Run("already gone", func(t *testing.T) {
		t.Parallel()

		target := NewSettingManagerBuilder(c, nil, nil, nil, nil)
		id := faker.Word()

		target.Release(connectionName, id)

		assert.Empty(t, target.inProgress)
	})
}
