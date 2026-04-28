package budgetfilter

import (
	"fmt"
	"strconv"
	"testing"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/config"
	budgetdatamock "github.com/mydecisive/octant/internal/mock/budgetdata"
	kubernetesmock "github.com/mydecisive/octant/internal/mock/kubernetes"
	kubeappsv1mock "github.com/mydecisive/octant/internal/mock/kubernetes/appsv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
)

func TestMDAISettingController_GetFilter(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	connection := faker.Word()
	expectedPct := 10
	expectedIncludeErr := true

	c := &config.Configuration{}

	t.Run("success log", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().
			GetVariable(namespace, connection, varLogsRatioNumber).
			Return(strconv.Itoa(expectedPct), nil).
			Once()
		mockAccessor.EXPECT().
			GetVariable(namespace, connection, varLogsPersistErrors).
			Return(strconv.FormatBool(expectedIncludeErr), nil).
			Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		require.NoError(t, err)

		assert.Equal(t, uint32(expectedPct), actual.GetPctSampled())
		assert.Equal(t, expectedIncludeErr, actual.GetIncludeErr())
	})

	t.Run("success trace", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().
			GetVariable(namespace, connection, varTracesRatioNumber).
			Return(strconv.Itoa(expectedPct), nil).
			Once()
		mockAccessor.EXPECT().
			GetVariable(namespace, connection, varTracesPersistErrors).
			Return(strconv.FormatBool(expectedIncludeErr), nil).
			Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_TRACE, namespace, connection)
		require.NoError(t, err)

		assert.Equal(t, uint32(expectedPct), actual.GetPctSampled())
		assert.Equal(t, expectedIncludeErr, actual.GetIncludeErr())
	})

	t.Run("err log lock in use", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)

		target := NewMDAISettingController(c, mockAccessor, nil)
		target.log.Lock()

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		require.ErrorIs(t, err, ErrStillUpdating)
		assert.Nil(t, actual)
	})

	t.Run("err trace lock in use", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)

		target := NewMDAISettingController(c, mockAccessor, nil)
		target.trace.Lock()

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_TRACE, namespace, connection)
		require.ErrorIs(t, err, ErrStillUpdating)
		assert.Nil(t, actual)
	})

	t.Run("err ratio num not found", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsRatioNumber).Return("", assert.AnError).Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		require.ErrorIs(t, err, ErrNotFound)
		assert.Nil(t, actual)
	})

	t.Run("err ratio num invalid", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsRatioNumber).Return(faker.Word(), nil).Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		require.ErrorIs(t, err, ErrFormat)
		assert.Nil(t, actual)
	})

	t.Run("err include err not found", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().
			GetVariable(namespace, connection, varLogsRatioNumber).
			Return(strconv.Itoa(expectedPct), nil).
			Once()
		mockAccessor.EXPECT().
			GetVariable(namespace, connection, varLogsPersistErrors).
			Return("", assert.AnError).
			Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		require.ErrorIs(t, err, ErrNotFound)
		assert.Nil(t, actual)
	})

	t.Run("err include err invalid", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().
			GetVariable(namespace, connection, varLogsRatioNumber).
			Return(strconv.Itoa(expectedPct), nil).
			Once()
		mockAccessor.EXPECT().
			GetVariable(namespace, connection, varLogsPersistErrors).
			Return(strconv.Itoa(expectedPct), nil).
			Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		require.ErrorIs(t, err, ErrFormat)
		assert.Nil(t, actual)
	})
}

func TestMDAISettingController_UpdateFilter(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	connection := faker.Word()

	c := &config.Configuration{
		Budget: config.Budget{
			FilterSettingUpdateTimeout:  1,
			FilterSettingUpdateInterval: 1,
		},
	}

	t.Run("success log", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_LOG,
			PctSampled: 10,
			IncludeErr: true,
		}

		controllerName := fmt.Sprintf(collectorLogNameFormatter, connection)

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varLogsRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varLogsPersistErrors, mock.Anything).Return(nil).Once()
		mockKube := kubernetesmock.NewMockInterface(t)
		mockAppsv1 := kubeappsv1mock.NewMockAppsV1Interface(t)
		mockDeployment := kubeappsv1mock.NewMockDeploymentInterface(t)
		mockKube.EXPECT().AppsV1().Return(mockAppsv1)
		mockAppsv1.EXPECT().Deployments(namespace).Return(mockDeployment)
		mockDeployment.EXPECT().Get(mock.Anything, controllerName, mock.Anything).Return(&appsv1.Deployment{
			Status: appsv1.DeploymentStatus{
				Replicas:        1,
				ReadyReplicas:   1,
				UpdatedReplicas: 1,
			},
		}, nil)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		actual := make(chan UpdateFilterResult)
		go func() {
			target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
		}()

		count := 0
		for result := range actual {
			switch count {
			case 0:
				assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_VALUE_UPDATED, result.Status)
			case 1:
				assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_WAIT_PROPAGATION, result.Status)
			case 2:
				assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_COMPLETED, result.Status)
			}
			assert.NoError(t, result.Err)
			count++
		}
	})

	t.Run("success trace", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
			PctSampled: 10,
			IncludeErr: true,
		}

		controllerName := fmt.Sprintf(collectorTraceNameFormatter, connection)

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesPersistErrors, mock.Anything).Return(nil).Once()
		mockKube := kubernetesmock.NewMockInterface(t)
		mockAppsv1 := kubeappsv1mock.NewMockAppsV1Interface(t)
		mockDeployment := kubeappsv1mock.NewMockDeploymentInterface(t)
		mockKube.EXPECT().AppsV1().Return(mockAppsv1)
		mockAppsv1.EXPECT().Deployments(namespace).Return(mockDeployment)
		mockDeployment.EXPECT().Get(mock.Anything, controllerName, mock.Anything).Return(&appsv1.Deployment{
			Status: appsv1.DeploymentStatus{
				Replicas:        1,
				ReadyReplicas:   1,
				UpdatedReplicas: 1,
			},
		}, nil)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		actual := make(chan UpdateFilterResult)
		go func() {
			target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
		}()

		count := 0
		for result := range actual {
			switch count {
			case 0:
				assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_VALUE_UPDATED, result.Status)
			case 1:
				assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_WAIT_PROPAGATION, result.Status)
			case 2:
				assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_COMPLETED, result.Status)
			}
			assert.NoError(t, result.Err)
			count++
		}
	})

	t.Run("err log lock in use", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_LOG,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)
		target.log.Lock()

		actual := make(chan UpdateFilterResult)
		go func() {
			target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
		}()

		actualData := <-actual
		assert.Empty(t, actualData.Status)
		assert.ErrorIs(t, actualData.Err, ErrStillUpdating)
	})

	t.Run("err trace lock in use", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)
		target.trace.Lock()

		actual := make(chan UpdateFilterResult)
		go func() {
			target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
		}()

		actualData := <-actual
		assert.Empty(t, actualData.Status)
		assert.ErrorIs(t, actualData.Err, ErrStillUpdating)
	})

	t.Run("err invalid type", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_UNSPECIFIED,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)
		target.trace.Lock()

		actual := make(chan UpdateFilterResult)
		go func() {
			target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
		}()

		actualData := <-actual
		assert.Empty(t, actualData.Status)
		assert.ErrorIs(t, actualData.Err, ErrInvalid)
	})

	t.Run("err update ratio num", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_LOG,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().
			UpdateVariable(namespace, connection, varLogsRatioNumber, mock.Anything).
			Return(assert.AnError).
			Once()
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		actual := make(chan UpdateFilterResult)
		go func() {
			target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
		}()

		actualData := <-actual
		assert.Empty(t, actualData.Status)
		assert.ErrorIs(t, actualData.Err, ErrUpdateValue)
	})

	t.Run("err update include err", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_LOG,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varLogsRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().
			UpdateVariable(namespace, connection, varLogsPersistErrors, mock.Anything).
			Return(assert.AnError).
			Once()
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		actual := make(chan UpdateFilterResult)
		go func() {
			target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
		}()

		actualData := <-actual
		assert.Empty(t, actualData.Status)
		assert.ErrorIs(t, actualData.Err, ErrUpdateValue)
	})

	t.Run("err get deployment", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
			PctSampled: 10,
			IncludeErr: true,
		}

		controllerName := fmt.Sprintf(collectorTraceNameFormatter, connection)

		mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesPersistErrors, mock.Anything).Return(nil).Once()
		mockKube := kubernetesmock.NewMockInterface(t)
		mockAppsv1 := kubeappsv1mock.NewMockAppsV1Interface(t)
		mockDeployment := kubeappsv1mock.NewMockDeploymentInterface(t)
		mockKube.EXPECT().AppsV1().Return(mockAppsv1)
		mockAppsv1.EXPECT().Deployments(namespace).Return(mockDeployment)
		mockDeployment.EXPECT().Get(mock.Anything, controllerName, mock.Anything).Return(nil, assert.AnError).Once()

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		actual := make(chan UpdateFilterResult)
		go func() {
			target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
		}()

		count := 0
		for result := range actual {
			switch count {
			case 0:
				require.NoError(t, result.Err)
				assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_VALUE_UPDATED, result.Status)
			case 1:
				require.NoError(t, result.Err)
				assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_WAIT_PROPAGATION, result.Status)
			case 2:
				assert.Empty(t, result.Status)
				assert.ErrorIs(t, result.Err, ErrUpdateCollector)
			}
			count++
		}
	})
}

// nolint: paralleltest // can't be parallel
func TestMDAISettingController_UpdateFilterTimeout(t *testing.T) {
	namespace := faker.Word()
	connection := faker.Word()

	c := &config.Configuration{
		Budget: config.Budget{
			FilterSettingUpdateTimeout:  1,
			FilterSettingUpdateInterval: 1,
		},
	}

	input := budgetv1alpha.Filter{
		Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
		PctSampled: 10,
		IncludeErr: true,
	}

	controllerName := fmt.Sprintf(collectorTraceNameFormatter, connection)

	mockAccessor := budgetdatamock.NewMockVariableAccessor(t)
	mockAccessor.EXPECT().
		UpdateVariable(namespace, connection, varTracesRatioNumber, mock.Anything).
		Return(nil).
		Once()
	mockAccessor.EXPECT().
		UpdateVariable(namespace, connection, varTracesPersistErrors, mock.Anything).
		Return(nil).
		Once()
	mockKube := kubernetesmock.NewMockInterface(t)
	mockAppsv1 := kubeappsv1mock.NewMockAppsV1Interface(t)
	mockDeployment := kubeappsv1mock.NewMockDeploymentInterface(t)
	mockKube.EXPECT().AppsV1().Return(mockAppsv1)
	mockAppsv1.EXPECT().Deployments(namespace).Return(mockDeployment)
	mockDeployment.EXPECT().Get(mock.Anything, controllerName, mock.Anything).Return(&appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			Replicas:      2,
			ReadyReplicas: 1,
		},
	}, nil)

	target := NewMDAISettingController(c, mockAccessor, mockKube)

	actual := make(chan UpdateFilterResult)
	go func() {
		target.UpdateFilter(t.Context(), namespace, connection, &input, actual)
	}()

	count := 0
	for result := range actual {
		switch count {
		case 0:
			require.NoError(t, result.Err)
			assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_VALUE_UPDATED, result.Status)
		case 1:
			require.NoError(t, result.Err)
			assert.Equal(t, budgetv1alpha.UpdateFilterResponse_STATUS_WAIT_PROPAGATION, result.Status)
		case 3:
			assert.Empty(t, result.Status)
			assert.ErrorIs(t, result.Err, ErrTimeout)
		}
		count++
	}
}
