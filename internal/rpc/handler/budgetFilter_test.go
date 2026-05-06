package rpchandler

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha/budgetv1alphaconnect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/go-faker/faker/v4/pkg/options"
	budgetfilter "github.com/mydecisive/octant/internal/budget/filter"
	"github.com/mydecisive/octant/internal/connection"
	budgetfiltermock "github.com/mydecisive/octant/internal/mock/budgetfilter"
	connectionmock "github.com/mydecisive/octant/internal/mock/connection"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBudgetFilterHandler_GetFilter(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	conn := faker.Word()

	var task *octantv1alpha.SaveDatadogIntegrationRequest
	require.NoError(t, faker.FakeData(&task, options.WithRandomMapAndSliceMaxSize(1)))

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		filterType := budgetv1alpha.FilterType_FILTER_TYPE_LOG

		var expected *budgetv1alpha.Filter
		require.NoError(t, faker.FakeData(&expected, options.WithRandomMapAndSliceMaxSize(1)))

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Logs},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().GetFilter(filterType, namespace, conn).Return(expected, nil)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)

		actual, err := target.GetFilter(t.Context(), connect.NewRequest(&budgetv1alpha.GetFilterRequest{
			Type:           filterType,
			Namespace:      namespace,
			ConnectionName: conn,
		}))
		require.NoError(t, err)

		assert.Equal(t, filterType, actual.Msg.GetData().GetType())
		assert.Equal(t, expected.GetPctSampled(), actual.Msg.GetData().GetPctSampled())
		assert.Equal(t, expected.GetIncludeErr(), actual.Msg.GetData().GetIncludeErr())
	})

	t.Run("err invalid", func(t *testing.T) {
		t.Parallel()

		filterType := budgetv1alpha.FilterType_FILTER_TYPE_TRACE

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().GetFilter(filterType, namespace, conn).Return(nil, budgetfilter.ErrInvalid)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)

		actual, err := target.GetFilter(t.Context(), connect.NewRequest(&budgetv1alpha.GetFilterRequest{
			Type:           filterType,
			Namespace:      namespace,
			ConnectionName: conn,
		}))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("err formatting", func(t *testing.T) {
		t.Parallel()

		filterType := budgetv1alpha.FilterType_FILTER_TYPE_TRACE

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().GetFilter(filterType, namespace, conn).Return(nil, budgetfilter.ErrFormat)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)

		actual, err := target.GetFilter(t.Context(), connect.NewRequest(&budgetv1alpha.GetFilterRequest{
			Type:           filterType,
			Namespace:      namespace,
			ConnectionName: conn,
		}))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("err not found", func(t *testing.T) {
		t.Parallel()

		filterType := budgetv1alpha.FilterType_FILTER_TYPE_TRACE

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().GetFilter(filterType, namespace, conn).Return(nil, budgetfilter.ErrNotFound)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)

		actual, err := target.GetFilter(t.Context(), connect.NewRequest(&budgetv1alpha.GetFilterRequest{
			Type:           filterType,
			Namespace:      namespace,
			ConnectionName: conn,
		}))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("err still updating", func(t *testing.T) {
		t.Parallel()

		filterType := budgetv1alpha.FilterType_FILTER_TYPE_TRACE

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().GetFilter(filterType, namespace, conn).Return(nil, budgetfilter.ErrStillUpdating)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)

		actual, err := target.GetFilter(t.Context(), connect.NewRequest(&budgetv1alpha.GetFilterRequest{
			Type:           filterType,
			Namespace:      namespace,
			ConnectionName: conn,
		}))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnavailable, connectErr.Code())
	})

	t.Run("err no connection", func(t *testing.T) {
		t.Parallel()

		filterType := budgetv1alpha.FilterType_FILTER_TYPE_TRACE

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(nil, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)

		actual, err := target.GetFilter(t.Context(), connect.NewRequest(&budgetv1alpha.GetFilterRequest{
			Type:           filterType,
			Namespace:      namespace,
			ConnectionName: conn,
		}))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("err wrong type", func(t *testing.T) {
		t.Parallel()

		filterType := budgetv1alpha.FilterType_FILTER_TYPE_TRACE

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Logs},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)

		actual, err := target.GetFilter(t.Context(), connect.NewRequest(&budgetv1alpha.GetFilterRequest{
			Type:           filterType,
			Namespace:      namespace,
			ConnectionName: conn,
		}))
		assert.Nil(t, actual)
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestBudgetFilterHandler_UpdateFilter(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	conn := faker.Word()
	input := &budgetv1alpha.Filter{
		Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
		PctSampled: 10,
		IncludeErr: true,
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().UpdateFilter(
			mock.Anything,
			namespace,
			conn,
			mock.Anything,
			mock.Anything,
		).Run(func(_ context.Context, _, _ string, _ *budgetv1alpha.Filter, out chan budgetfilter.UpdateFilterResult) {
			out <- budgetfilter.UpdateFilterResult{
				Status: budgetv1alpha.UpdateFilterResponse_STATUS_COMPLETED,
			}
			close(out)
		})

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		target := NewBudgetFilterHandler(mockConn, mockCtrl)
		_, handler := budgetv1alphaconnect.NewFilterServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := budgetv1alphaconnect.NewFilterServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.UpdateFilter(t.Context(), connect.NewRequest(&budgetv1alpha.UpdateFilterRequest{
			Namespace:      namespace,
			ConnectionName: conn,
			Data:           input,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		require.True(t, stream.Receive())
		assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_COMPLETED, stream.Msg().GetStatus())
	})

	t.Run("err invalid", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().UpdateFilter(
			mock.Anything,
			namespace,
			conn,
			mock.Anything,
			mock.Anything,
		).Run(func(_ context.Context, _, _ string, _ *budgetv1alpha.Filter, out chan budgetfilter.UpdateFilterResult) {
			out <- budgetfilter.UpdateFilterResult{
				Err: budgetfilter.ErrInvalid,
			}
			close(out)
		})

		target := NewBudgetFilterHandler(mockConn, mockCtrl)
		_, handler := budgetv1alphaconnect.NewFilterServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := budgetv1alphaconnect.NewFilterServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.UpdateFilter(t.Context(), connect.NewRequest(&budgetv1alpha.UpdateFilterRequest{
			Namespace:      namespace,
			ConnectionName: conn,
			Data:           input,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		stream.Receive()
		var connectErr *connect.Error
		require.ErrorAs(t, stream.Err(), &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("err still updating", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().UpdateFilter(
			mock.Anything,
			namespace,
			conn,
			mock.Anything,
			mock.Anything,
		).Run(func(_ context.Context, _, _ string, _ *budgetv1alpha.Filter, out chan budgetfilter.UpdateFilterResult) {
			out <- budgetfilter.UpdateFilterResult{
				Err: budgetfilter.ErrStillUpdating,
			}
			close(out)
		})

		target := NewBudgetFilterHandler(mockConn, mockCtrl)
		_, handler := budgetv1alphaconnect.NewFilterServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := budgetv1alphaconnect.NewFilterServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.UpdateFilter(t.Context(), connect.NewRequest(&budgetv1alpha.UpdateFilterRequest{
			Namespace:      namespace,
			ConnectionName: conn,
			Data:           input,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		stream.Receive()
		var connectErr *connect.Error
		require.ErrorAs(t, stream.Err(), &connectErr)
		assert.Equal(t, connect.CodeUnavailable, connectErr.Code())
	})

	t.Run("err update values", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().UpdateFilter(
			mock.Anything,
			namespace,
			conn,
			mock.Anything,
			mock.Anything,
		).Run(func(_ context.Context, _, _ string, _ *budgetv1alpha.Filter, out chan budgetfilter.UpdateFilterResult) {
			out <- budgetfilter.UpdateFilterResult{
				Err: budgetfilter.ErrUpdateValue,
			}
			close(out)
		})

		target := NewBudgetFilterHandler(mockConn, mockCtrl)
		_, handler := budgetv1alphaconnect.NewFilterServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := budgetv1alphaconnect.NewFilterServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.UpdateFilter(t.Context(), connect.NewRequest(&budgetv1alpha.UpdateFilterRequest{
			Namespace:      namespace,
			ConnectionName: conn,
			Data:           input,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		stream.Receive()
		var connectErr *connect.Error
		require.ErrorAs(t, stream.Err(), &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
		assert.Contains(t, connectErr.Error(), budgetfilter.ErrUpdateValue.Error())
	})

	t.Run("err update values", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Traces},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)
		mockCtrl.EXPECT().UpdateFilter(
			mock.Anything,
			namespace,
			conn,
			mock.Anything,
			mock.Anything,
		).Run(func(_ context.Context, _, _ string, _ *budgetv1alpha.Filter, out chan budgetfilter.UpdateFilterResult) {
			out <- budgetfilter.UpdateFilterResult{
				Err: budgetfilter.ErrUpdateCollector,
			}
			close(out)
		})

		target := NewBudgetFilterHandler(mockConn, mockCtrl)
		_, handler := budgetv1alphaconnect.NewFilterServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := budgetv1alphaconnect.NewFilterServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.UpdateFilter(t.Context(), connect.NewRequest(&budgetv1alpha.UpdateFilterRequest{
			Namespace:      namespace,
			ConnectionName: conn,
			Data:           input,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		stream.Receive()
		var connectErr *connect.Error
		require.ErrorAs(t, stream.Err(), &connectErr)
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
		assert.Contains(t, connectErr.Error(), budgetfilter.ErrUpdateCollector.Error())
	})

	t.Run("err wrong type", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(&connection.OctantConnectionData{
			TelemetryTypes: []telemetry.MLT{telemetry.Logs},
		}, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)
		_, handler := budgetv1alphaconnect.NewFilterServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := budgetv1alphaconnect.NewFilterServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.UpdateFilter(t.Context(), connect.NewRequest(&budgetv1alpha.UpdateFilterRequest{
			Namespace:      namespace,
			ConnectionName: conn,
			Data:           input,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		stream.Receive()
		var connectErr *connect.Error
		require.ErrorAs(t, stream.Err(), &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("err no connection", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			namespace,
			conn,
		).Return(nil, nil).Once()

		mockCtrl := budgetfiltermock.NewMockSettingController(t)

		target := NewBudgetFilterHandler(mockConn, mockCtrl)
		_, handler := budgetv1alphaconnect.NewFilterServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := budgetv1alphaconnect.NewFilterServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.UpdateFilter(t.Context(), connect.NewRequest(&budgetv1alpha.UpdateFilterRequest{
			Namespace:      namespace,
			ConnectionName: conn,
			Data:           input,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		stream.Receive()
		var connectErr *connect.Error
		require.ErrorAs(t, stream.Err(), &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}
