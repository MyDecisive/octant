package rpchandler

import (
	"testing"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/go-faker/faker/v4/pkg/options"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	budgetmock "github.com/mydecisive/octant/internal/mock/budget"
	connectionmock "github.com/mydecisive/octant/internal/mock/connection"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBudgetHandler_Overall(t *testing.T) {
	t.Parallel()

	task := &budgetv1alpha.OverallRequest{
		Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_MTD,
		Namespace: faker.Word(),
	}
	conf := config.Configuration{
		CurrentNamespace: faker.Word(),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var expected *budgetv1alpha.Overall
		require.NoError(t, faker.FakeData(&expected, options.WithRandomMapAndSliceMaxSize(1)))

		mockProvider := budgetmock.NewMockMetricDataProvider(t)
		mockProvider.EXPECT().GetOverall(
			mock.Anything,
			task.GetTimeframe(),
			task.GetNamespace(),
		).Return(expected, nil).Once()

		target := NewBudgetHandler(conf, nil, mockProvider)

		actual, err := target.Overall(t.Context(), connect.NewRequest(task))
		require.NoError(t, err)

		assert.Equal(t, expected, actual.Msg.GetData())
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockProvider := budgetmock.NewMockMetricDataProvider(t)
		mockProvider.EXPECT().GetOverall(
			mock.Anything,
			task.GetTimeframe(),
			task.GetNamespace(),
		).Return(nil, assert.AnError).Once()

		target := NewBudgetHandler(conf, nil, mockProvider)

		actual, err := target.Overall(t.Context(), connect.NewRequest(task))
		assert.Nil(t, actual)
		assert.Error(t, err)
	})
}

func TestBudgetHandler_Log(t *testing.T) {
	t.Parallel()

	var task *budgetv1alpha.LogRequest
	require.NoError(t, faker.FakeData(&task, options.WithRandomMapAndSliceMaxSize(1)))
	task.Timeframe = budgetv1alpha.Timeframe_TIMEFRAME_MTD
	conf := config.Configuration{
		CurrentNamespace: faker.Word(),
	}

	expectedInput := budgetdata.MetricDataInput{
		Timeframe: task.GetTimeframe(),
		Size:      task.GetSize(),
		PageToken: task.GetPageToken(),
		Search:    task.GetSearch(),
		Namespace: task.GetNamespace(),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var expected *budgetv1alpha.Log
		require.NoError(t, faker.FakeData(&expected, options.WithRandomMapAndSliceMaxSize(1)))
		expectedNext := faker.Word()

		mockProvider := budgetmock.NewMockMetricDataProvider(t)
		mockProvider.EXPECT().GetLogs(
			mock.Anything,
			mock.MatchedBy(func(input budgetdata.MetricDataInput) bool {
				return input.Timeframe == expectedInput.Timeframe &&
					input.Size == expectedInput.Size &&
					input.PageToken == expectedInput.PageToken &&
					input.Search == expectedInput.Search &&
					input.Namespace == expectedInput.Namespace
			}),
		).Return([]*budgetv1alpha.Log{expected}, expectedNext, nil).Once()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Logs},
		}, nil).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Log(t.Context(), connect.NewRequest(task))
		require.NoError(t, err)

		assert.Len(t, actual.Msg.GetData(), 1)
		assert.Equal(t, expected, actual.Msg.GetData()[0])
		assert.Equal(t, expectedNext, actual.Msg.GetNextPageToken())
	})

	t.Run("err not allowed", func(t *testing.T) {
		t.Parallel()

		mockProvider := budgetmock.NewMockMetricDataProvider(t)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Log(t.Context(), connect.NewRequest(task))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("err get connection", func(t *testing.T) {
		t.Parallel()

		mockProvider := budgetmock.NewMockMetricDataProvider(t)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(nil, assert.AnError).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Log(t.Context(), connect.NewRequest(task))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("err get data", func(t *testing.T) {
		t.Parallel()

		var expected *budgetv1alpha.Log
		require.NoError(t, faker.FakeData(&expected, options.WithRandomMapAndSliceMaxSize(1)))

		mockProvider := budgetmock.NewMockMetricDataProvider(t)
		mockProvider.EXPECT().GetLogs(
			mock.Anything,
			mock.MatchedBy(func(input budgetdata.MetricDataInput) bool {
				return input.Timeframe == expectedInput.Timeframe &&
					input.Size == expectedInput.Size &&
					input.PageToken == expectedInput.PageToken &&
					input.Search == expectedInput.Search &&
					input.Namespace == expectedInput.Namespace
			}),
		).Return(nil, "", assert.AnError).Once()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Logs},
		}, nil).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Log(t.Context(), connect.NewRequest(task))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestBudgetHandler_Trace(t *testing.T) {
	t.Parallel()

	var task *budgetv1alpha.TraceRequest
	require.NoError(t, faker.FakeData(&task, options.WithRandomMapAndSliceMaxSize(1)))
	task.Timeframe = budgetv1alpha.Timeframe_TIMEFRAME_MTD
	conf := config.Configuration{
		CurrentNamespace: faker.Word(),
	}

	expectedInput := budgetdata.MetricDataInput{
		Timeframe: task.GetTimeframe(),
		Size:      task.GetSize(),
		PageToken: task.GetPageToken(),
		Search:    task.GetSearch(),
		Namespace: task.GetNamespace(),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var expected *budgetv1alpha.Span
		require.NoError(t, faker.FakeData(&expected, options.WithRandomMapAndSliceMaxSize(1)))
		expectedNext := faker.Word()

		mockProvider := budgetmock.NewMockMetricDataProvider(t)
		mockProvider.EXPECT().GetSpans(
			mock.Anything,
			mock.MatchedBy(func(input budgetdata.MetricDataInput) bool {
				return input.Timeframe == expectedInput.Timeframe &&
					input.Size == expectedInput.Size &&
					input.PageToken == expectedInput.PageToken &&
					input.Search == expectedInput.Search &&
					input.Namespace == expectedInput.Namespace
			}),
		).Return([]*budgetv1alpha.Span{expected}, expectedNext, nil).Once()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Trace(t.Context(), connect.NewRequest(task))
		require.NoError(t, err)

		assert.Len(t, actual.Msg.GetData(), 1)
		assert.Equal(t, expected, actual.Msg.GetData()[0])
		assert.Equal(t, expectedNext, actual.Msg.GetNextPageToken())
	})

	t.Run("err not allowed", func(t *testing.T) {
		t.Parallel()

		mockProvider := budgetmock.NewMockMetricDataProvider(t)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Logs},
		}, nil).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Trace(t.Context(), connect.NewRequest(task))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("err no connection", func(t *testing.T) {
		t.Parallel()

		mockProvider := budgetmock.NewMockMetricDataProvider(t)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(nil, nil).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Trace(t.Context(), connect.NewRequest(task))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("err get connection", func(t *testing.T) {
		t.Parallel()

		mockProvider := budgetmock.NewMockMetricDataProvider(t)

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(nil, assert.AnError).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Trace(t.Context(), connect.NewRequest(task))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("err get data", func(t *testing.T) {
		t.Parallel()

		var expected *budgetv1alpha.Span
		require.NoError(t, faker.FakeData(&expected, options.WithRandomMapAndSliceMaxSize(1)))

		mockProvider := budgetmock.NewMockMetricDataProvider(t)
		mockProvider.EXPECT().GetSpans(
			mock.Anything,
			mock.MatchedBy(func(input budgetdata.MetricDataInput) bool {
				return input.Timeframe == expectedInput.Timeframe &&
					input.Size == expectedInput.Size &&
					input.PageToken == expectedInput.PageToken &&
					input.Search == expectedInput.Search &&
					input.Namespace == expectedInput.Namespace
			}),
		).Return(nil, "", assert.AnError).Once()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			conf.CurrentNamespace,
			task.GetConnectionName(),
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		target := NewBudgetHandler(conf, mockConn, mockProvider)

		actual, err := target.Trace(t.Context(), connect.NewRequest(task))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}
