package rpchandler

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/go-faker/faker/v4"
	settingmock "github.com/mydecisive/octant/internal/mock/setting"
	"github.com/mydecisive/octant/internal/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_Update(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	conn := faker.Word()
	newURL := faker.Word()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockManager := settingmock.NewMockManager(t)
		mockManager.EXPECT().SetDatadogURL(newURL).Return(mockManager).Once()
		mockManager.EXPECT().SetDatadogAPIKey("").Return(mockManager).Once()
		mockManager.EXPECT().SetTelemetryTypes([]octantv1alpha.MLTType(nil)).Return(mockManager).Once()
		mockManager.EXPECT().Apply(mock.Anything).Return(nil).Once()
		mockManager.EXPECT().DeployAndWait(mock.Anything, mock.Anything).Run(func(_ context.Context,
			out chan setting.SettingUpdateResult,
		) {
			out <- setting.SettingUpdateResult{
				Status: octantv1alpha.UpdateResponse_STATUS_COMPLETED,
			}
			close(out)
		})

		mockBuilder := settingmock.NewMockManagerBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace, conn, mock.Anything).Return(mockManager, nil)

		target := NewSettingHandler(mockBuilder)
		_, handler := octantv1alphaconnect.NewSettingServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := octantv1alphaconnect.NewSettingServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.Update(t.Context(), connect.NewRequest(&octantv1alpha.UpdateRequest{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      namespace,
				ConnectionName: conn,
			},
			DatadogUrl: newURL,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		count := 0
		for ok := stream.Receive(); ok; ok = stream.Receive() {
			switch count {
			case 0:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_UPDATING, stream.Msg().GetStatus())
			case 1:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_UPDATED, stream.Msg().GetStatus())
			case 2:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_COMPLETED, stream.Msg().GetStatus())
			default:
				assert.Fail(t, "too many statuses")
			}
			assert.NoError(t, stream.Err())
			count++
		}
	})

	t.Run("err builder", func(t *testing.T) {
		t.Parallel()

		mockBuilder := settingmock.NewMockManagerBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace, conn, mock.Anything).Return(nil, assert.AnError)

		target := NewSettingHandler(mockBuilder)
		_, handler := octantv1alphaconnect.NewSettingServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := octantv1alphaconnect.NewSettingServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.Update(t.Context(), connect.NewRequest(&octantv1alpha.UpdateRequest{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      namespace,
				ConnectionName: conn,
			},
			DatadogUrl: newURL,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		stream.Receive()
		var connectErr *connect.Error
		require.ErrorAs(t, stream.Err(), &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("err apply", func(t *testing.T) {
		t.Parallel()

		mockManager := settingmock.NewMockManager(t)
		mockManager.EXPECT().SetDatadogURL(newURL).Return(mockManager).Once()
		mockManager.EXPECT().SetDatadogAPIKey("").Return(mockManager).Once()
		mockManager.EXPECT().SetTelemetryTypes([]octantv1alpha.MLTType(nil)).Return(mockManager).Once()
		mockManager.EXPECT().Apply(mock.Anything).Return(assert.AnError).Once()

		mockBuilder := settingmock.NewMockManagerBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace, conn, mock.Anything).Return(mockManager, nil)

		target := NewSettingHandler(mockBuilder)
		_, handler := octantv1alphaconnect.NewSettingServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := octantv1alphaconnect.NewSettingServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.Update(t.Context(), connect.NewRequest(&octantv1alpha.UpdateRequest{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      namespace,
				ConnectionName: conn,
			},
			DatadogUrl: newURL,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		count := 0
		for ok := stream.Receive(); ok; ok = stream.Receive() {
			switch count {
			case 0:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_UPDATING, stream.Msg().GetStatus())
				require.NoError(t, stream.Err())
			case 1:
				var connectErr *connect.Error
				require.ErrorAs(t, stream.Err(), &connectErr)
				assert.Equal(t, connect.CodeInternal, connectErr.Code())
			default:
				assert.Fail(t, "too many statuses")
			}
			count++
		}
	})

	t.Run("err deploy", func(t *testing.T) {
		t.Parallel()

		mockManager := settingmock.NewMockManager(t)
		mockManager.EXPECT().SetDatadogURL(newURL).Return(mockManager).Once()
		mockManager.EXPECT().SetDatadogAPIKey("").Return(mockManager).Once()
		mockManager.EXPECT().SetTelemetryTypes([]octantv1alpha.MLTType(nil)).Return(mockManager).Once()
		mockManager.EXPECT().Apply(mock.Anything).Return(nil).Once()
		mockManager.EXPECT().DeployAndWait(mock.Anything, mock.Anything).Run(func(_ context.Context,
			out chan setting.SettingUpdateResult,
		) {
			out <- setting.SettingUpdateResult{
				Err: setting.ErrDeploy,
			}
			close(out)
		})

		mockBuilder := settingmock.NewMockManagerBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace, conn, mock.Anything).Return(mockManager, nil)

		target := NewSettingHandler(mockBuilder)
		_, handler := octantv1alphaconnect.NewSettingServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := octantv1alphaconnect.NewSettingServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.Update(t.Context(), connect.NewRequest(&octantv1alpha.UpdateRequest{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      namespace,
				ConnectionName: conn,
			},
			DatadogUrl: newURL,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		count := 0
		for ok := stream.Receive(); ok; ok = stream.Receive() {
			switch count {
			case 0:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_UPDATING, stream.Msg().GetStatus())
				require.NoError(t, stream.Err())
			case 1:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_UPDATED, stream.Msg().GetStatus())
				require.NoError(t, stream.Err())
			case 2:
				var connectErr *connect.Error
				require.ErrorAs(t, stream.Err(), &connectErr)
				assert.Equal(t, connect.CodeInternal, connectErr.Code())
			default:
				assert.Fail(t, "too many statuses")
			}
			count++
		}
	})

	t.Run("err wait", func(t *testing.T) {
		t.Parallel()

		mockManager := settingmock.NewMockManager(t)
		mockManager.EXPECT().SetDatadogURL(newURL).Return(mockManager).Once()
		mockManager.EXPECT().SetDatadogAPIKey("").Return(mockManager).Once()
		mockManager.EXPECT().SetTelemetryTypes([]octantv1alpha.MLTType(nil)).Return(mockManager).Once()
		mockManager.EXPECT().Apply(mock.Anything).Return(nil).Once()
		mockManager.EXPECT().DeployAndWait(mock.Anything, mock.Anything).Run(func(_ context.Context,
			out chan setting.SettingUpdateResult,
		) {
			out <- setting.SettingUpdateResult{
				Err: assert.AnError,
			}
			close(out)
		})

		mockBuilder := settingmock.NewMockManagerBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace, conn, mock.Anything).Return(mockManager, nil)

		target := NewSettingHandler(mockBuilder)
		_, handler := octantv1alphaconnect.NewSettingServiceHandler(target)

		testServer := httptest.NewUnstartedServer(handler)
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
		t.Cleanup(testServer.Close)

		client := octantv1alphaconnect.NewSettingServiceClient(testServer.Client(), testServer.URL)
		stream, err := client.Update(t.Context(), connect.NewRequest(&octantv1alpha.UpdateRequest{
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      namespace,
				ConnectionName: conn,
			},
			DatadogUrl: newURL,
		}))
		require.NoError(t, err)
		require.NotNil(t, stream)

		count := 0
		for ok := stream.Receive(); ok; ok = stream.Receive() {
			switch count {
			case 0:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_UPDATING, stream.Msg().GetStatus())
				require.NoError(t, stream.Err())
			case 1:
				assert.Equal(t, octantv1alpha.UpdateResponse_STATUS_UPDATED, stream.Msg().GetStatus())
				require.NoError(t, stream.Err())
			case 2:
				var connectErr *connect.Error
				require.ErrorAs(t, stream.Err(), &connectErr)
				assert.Equal(t, connect.CodeAborted, connectErr.Code())
			default:
				assert.Fail(t, "too many statuses")
			}
			count++
		}
	})
}
