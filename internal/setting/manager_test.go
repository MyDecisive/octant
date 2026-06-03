package setting

import (
	"context"
	"testing"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	connectionmock "github.com/mydecisive/octant/internal/mock/connection"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSettingManager_ID(t *testing.T) {
	t.Parallel()

	target := newTestManager(t, nil, nil, nil)
	actual := target.ID()

	assert.Equal(t, target.id, actual)
}

func TestSettingManager_SetDatadogURL(t *testing.T) {
	t.Parallel()

	t.Run("changed", func(t *testing.T) {
		t.Parallel()

		expected := "diff"
		target := newTestManager(t, nil, nil, nil)
		actual := target.SetDatadogURL(expected)
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Equal(t, expected, actualManager.datadog.DDUrl)
		assert.True(t, actualManager.shouldUpdateDatadog)
		assert.False(t, actualManager.shouldUpdateConnection)
	})

	t.Run("no change", func(t *testing.T) {
		t.Parallel()

		target := newTestManager(t, nil, nil, nil)
		expected := target.datadog.DDUrl
		actual := target.SetDatadogURL(target.datadog.DDUrl)
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Equal(t, expected, actualManager.datadog.DDUrl)
		assert.False(t, actualManager.shouldUpdateDatadog)
		assert.False(t, actualManager.shouldUpdateConnection)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		target := newTestManager(t, nil, nil, nil)
		expected := target.datadog.DDUrl
		actual := target.SetDatadogURL("")
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Equal(t, expected, actualManager.datadog.DDUrl)
		assert.False(t, actualManager.shouldUpdateDatadog)
		assert.False(t, actualManager.shouldUpdateConnection)
	})
}

func TestSettingManager_SetDatadogAPIKey(t *testing.T) {
	t.Parallel()

	t.Run("changed", func(t *testing.T) {
		t.Parallel()

		expected := "diff"
		target := newTestManager(t, nil, nil, nil)
		actual := target.SetDatadogAPIKey(expected)
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Equal(t, expected, actualManager.datadog.APIKey)
		assert.True(t, actualManager.shouldUpdateDatadog)
		assert.False(t, actualManager.shouldUpdateConnection)
	})

	t.Run("no change", func(t *testing.T) {
		t.Parallel()

		target := newTestManager(t, nil, nil, nil)
		expected := target.datadog.APIKey
		actual := target.SetDatadogAPIKey(target.datadog.APIKey)
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Equal(t, expected, actualManager.datadog.APIKey)
		assert.False(t, actualManager.shouldUpdateDatadog)
		assert.False(t, actualManager.shouldUpdateConnection)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		target := newTestManager(t, nil, nil, nil)
		expected := target.datadog.APIKey
		actual := target.SetDatadogAPIKey("")
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Equal(t, expected, actualManager.datadog.APIKey)
		assert.False(t, actualManager.shouldUpdateDatadog)
		assert.False(t, actualManager.shouldUpdateConnection)
	})
}

func TestSettingManager_SetTelemetryTypes(t *testing.T) {
	t.Parallel()

	t.Run("changed", func(t *testing.T) {
		t.Parallel()

		input := []octantv1alpha.MLTType{
			octantv1alpha.MLTType_MLT_TYPE_LOG,
		}
		target := newTestManager(t, nil, nil, nil)
		actual := target.SetTelemetryTypes(input)
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Len(t, actualManager.connection.TelemetryTypes, 1)
		assert.Contains(t, actualManager.connection.TelemetryTypes, telemetry.Logs)
		assert.False(t, actualManager.shouldUpdateDatadog)
		assert.True(t, actualManager.shouldUpdateConnection)
	})

	t.Run("no change", func(t *testing.T) {
		t.Parallel()

		input := []octantv1alpha.MLTType{
			octantv1alpha.MLTType_MLT_TYPE_LOG,
			octantv1alpha.MLTType_MLT_TYPE_TRACE,
		}
		target := newTestManager(t, nil, nil, nil)
		actual := target.SetTelemetryTypes(input)
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Len(t, actualManager.connection.TelemetryTypes, 2)
		assert.Contains(t, actualManager.connection.TelemetryTypes, telemetry.Logs)
		assert.Contains(t, actualManager.connection.TelemetryTypes, telemetry.Traces)
		assert.False(t, actualManager.shouldUpdateDatadog)
		assert.False(t, actualManager.shouldUpdateConnection)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		target := newTestManager(t, nil, nil, nil)
		actual := target.SetTelemetryTypes([]octantv1alpha.MLTType{})
		actualManager, ok := actual.(*SettingManager)
		require.True(t, ok)

		assert.Len(t, actualManager.connection.TelemetryTypes, 2)
		assert.False(t, actualManager.shouldUpdateDatadog)
		assert.False(t, actualManager.shouldUpdateConnection)
	})
}

func TestSettingManager_Apply(t *testing.T) {
	t.Parallel()

	t.Run("success both", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		target := newTestManager(t, mockConn, mockDatadog, nil)

		mockDatadog.EXPECT().SetIntegration(mock.Anything, target.connectionName, *target.datadog).Return(nil).Once()
		mockConn.EXPECT().SaveConnection(mock.Anything, *target.connection, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == target.namespace && in.ConnectionName == target.connectionName && in.NoDeploy
		})).Return(nil).Once()

		target.shouldUpdateDatadog = true
		target.shouldUpdateConnection = true

		err := target.Apply(t.Context())
		require.NoError(t, err)
	})

	t.Run("success only datadog", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		target := newTestManager(t, mockConn, mockDatadog, nil)

		mockDatadog.EXPECT().SetIntegration(mock.Anything, target.connectionName, *target.datadog).Return(nil).Once()

		target.shouldUpdateDatadog = true

		err := target.Apply(t.Context())
		require.NoError(t, err)
	})

	t.Run("success only connection", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		target := newTestManager(t, mockConn, mockDatadog, nil)

		mockConn.EXPECT().SaveConnection(mock.Anything, *target.connection, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == target.namespace && in.ConnectionName == target.connectionName && in.NoDeploy
		})).Return(nil).Once()

		target.shouldUpdateConnection = true

		err := target.Apply(t.Context())
		require.NoError(t, err)
	})

	t.Run("err datadog", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		target := newTestManager(t, mockConn, mockDatadog, nil)

		mockDatadog.EXPECT().SetIntegration(mock.Anything, target.connectionName, *target.datadog).Return(assert.AnError).Once()

		target.shouldUpdateDatadog = true
		target.shouldUpdateConnection = true

		err := target.Apply(t.Context())
		assert.Error(t, err)
	})

	t.Run("err connection", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		target := newTestManager(t, mockConn, mockDatadog, nil)

		mockDatadog.EXPECT().SetIntegration(mock.Anything, target.connectionName, *target.datadog).Return(nil).Once()
		mockConn.EXPECT().SaveConnection(mock.Anything, *target.connection, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == target.namespace && in.ConnectionName == target.connectionName && in.NoDeploy
		})).Return(assert.AnError).Once()

		target.shouldUpdateDatadog = true
		target.shouldUpdateConnection = true

		err := target.Apply(t.Context())
		assert.Error(t, err)
	})
}

func TestSettingManager_DeployAndWait(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)
		target := newTestManager(t, mockConn, nil, mockArgoClient)

		mockConn.EXPECT().SaveConnection(mock.Anything, *target.connection, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == target.namespace && in.ConnectionName == target.connectionName && in.Skip
		})).Return(nil).Once()
		mockArgoClient.EXPECT().AppOperationState(mock.Anything, mock.MatchedBy(func(in argocd.Input) bool {
			return in.ClientOpts.AuthToken == target.argo.AccountToken &&
				in.ClientOpts.ServerAddr == target.argo.APIUrl &&
				in.AppName == target.connectionName
		}), mock.Anything, mock.Anything, mock.Anything).Run(func(_ context.Context,
			_ argocd.Input,
			_ time.Duration,
			_ time.Duration,
			out chan argocd.InstallResult,
		) {
			out <- argocd.InstallResult{
				Status: octantv1alpha.InstallStatus_INSTALL_STATUS_INSTALLED,
			}
			close(out)
		})

		target.shouldUpdateDatadog = true
		target.shouldUpdateConnection = true

		actual := make(chan SettingUpdateResult)
		go target.DeployAndWait(t.Context(), actual)

		count := 0
		for result := range actual {
			switch count {
			case 0:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_DEPLOY, result.Status)
			case 1:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_COMPLETED, result.Status)
			default:
				assert.Fail(t, "too many statuses")
			}
			assert.NoError(t, result.Err)
			count++
		}
	})

	t.Run("success nothing to do", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)
		target := newTestManager(t, mockConn, nil, mockArgoClient)

		actual := make(chan SettingUpdateResult)
		go target.DeployAndWait(t.Context(), actual)

		count := 0
		for result := range actual {
			require.Less(t, count, 1)
			assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_COMPLETED, result.Status)
			assert.NoError(t, result.Err)
			count++
		}
	})

	t.Run("err deploy", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)
		target := newTestManager(t, mockConn, nil, mockArgoClient)

		mockConn.EXPECT().SaveConnection(mock.Anything, *target.connection, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == target.namespace && in.ConnectionName == target.connectionName && in.Skip
		})).Return(assert.AnError).Once()

		target.shouldUpdateDatadog = true

		actual := make(chan SettingUpdateResult)
		go target.DeployAndWait(t.Context(), actual)

		count := 0
		for result := range actual {
			switch count {
			case 0:
				require.NoError(t, result.Err)
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_DEPLOY, result.Status)
			case 1:
				assert.Empty(t, result.Status)
				require.Error(t, result.Err)
			default:
				assert.Fail(t, "too many statuses")
			}
			count++
		}
	})

	t.Run("err wait", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockArgoClient := argocdmock.NewMockAPIClient(t)
		target := newTestManager(t, mockConn, nil, mockArgoClient)

		mockConn.EXPECT().SaveConnection(mock.Anything, *target.connection, mock.MatchedBy(func(in connection.ConnectionCRUDInput) bool {
			return in.Namespace == target.namespace && in.ConnectionName == target.connectionName && in.Skip
		})).Return(nil).Once()
		mockArgoClient.EXPECT().AppOperationState(mock.Anything, mock.MatchedBy(func(in argocd.Input) bool {
			return in.ClientOpts.AuthToken == target.argo.AccountToken &&
				in.ClientOpts.ServerAddr == target.argo.APIUrl &&
				in.AppName == target.connectionName
		}), mock.Anything, mock.Anything, mock.Anything).Run(func(_ context.Context,
			_ argocd.Input,
			_ time.Duration,
			_ time.Duration,
			out chan argocd.InstallResult,
		) {
			out <- argocd.InstallResult{
				Err: assert.AnError,
			}
			close(out)
		})

		target.shouldUpdateConnection = true

		actual := make(chan SettingUpdateResult)
		go target.DeployAndWait(t.Context(), actual)

		count := 0
		for result := range actual {
			switch count {
			case 0:
				require.NoError(t, result.Err)
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_DEPLOY, result.Status)
			case 1:
				assert.Empty(t, result.Status)
				require.Error(t, result.Err)
			default:
				assert.Fail(t, "too many statuses")
			}
			count++
		}
	})
}

func newTestManager(
	t *testing.T,
	conn connection.Connection[connection.OctantConnectionData],
	dd integration.Integration[integration.DataDogIntegrationData],
	argo argocd.APIClient,
) *SettingManager {
	t.Helper()
	c := &config.Configuration{
		Install: config.Install{
			MdaiInstallPollingIntervalMillis: 1,
			MdaiInstallTimeout:               1,
		},
	}

	return &SettingManager{
		id:                faker.Word(),
		configuration:     c,
		connectionService: conn,
		datadogService:    dd,
		argoClient:        argo,
		logger:            zaptest.NewLogger(t),
		connectionName:    faker.Word(),
		namespace:         faker.Word(),
		connection: &connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{
				telemetry.Logs, telemetry.Traces,
			},
		},
		datadog: &integration.DataDogIntegrationData{
			DDUrl:  faker.URL(),
			APIKey: faker.Word(),
		},
		argo: &integration.ArgoCDIntegrationData{
			APIUrl:       faker.URL(),
			AccountToken: faker.Word(),
		},
		shouldUpdateDatadog:    false,
		shouldUpdateConnection: false,
	}
}
