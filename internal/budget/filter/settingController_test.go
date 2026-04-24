package budgetfilter

import (
	"fmt"
	"testing"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/config"
	budgetfiltermock "github.com/mydecisive/octant/internal/mock/budgetfilter"
	kubernetesmock "github.com/mydecisive/octant/internal/mock/kubernetes"
	kubeappsv1mock "github.com/mydecisive/octant/internal/mock/kubernetes/appsv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
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

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsRatioNumber).Return(fmt.Sprintf("%d", expectedPct), nil).Once()
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsPersistErrors).Return(fmt.Sprintf("%t", expectedIncludeErr), nil).Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		require.NoError(t, err)

		assert.Equal(t, uint32(expectedPct), actual.GetPctSampled())
		assert.Equal(t, expectedIncludeErr, actual.GetIncludeErr())
	})

	t.Run("success trace", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().GetVariable(namespace, connection, varTracesRatioNumber).Return(fmt.Sprintf("%d", expectedPct), nil).Once()
		mockAccessor.EXPECT().GetVariable(namespace, connection, varTracesPersistErrors).Return(fmt.Sprintf("%t", expectedIncludeErr), nil).Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_TRACE, namespace, connection)
		require.NoError(t, err)

		assert.Equal(t, uint32(expectedPct), actual.GetPctSampled())
		assert.Equal(t, expectedIncludeErr, actual.GetIncludeErr())
	})

	t.Run("err log lock in use", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)

		target := NewMDAISettingController(c, mockAccessor, nil)
		target.log.Lock()

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		assert.ErrorIs(t, err, ErrStillUpdating)
		assert.Nil(t, actual)
	})

	t.Run("err log lock in use", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)

		target := NewMDAISettingController(c, mockAccessor, nil)
		target.trace.Lock()

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_TRACE, namespace, connection)
		assert.ErrorIs(t, err, ErrStillUpdating)
		assert.Nil(t, actual)
	})

	t.Run("err ratio num not found", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsRatioNumber).Return("", assert.AnError).Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		assert.ErrorIs(t, err, ErrNotFound)
		assert.Nil(t, actual)
	})

	t.Run("err ratio num invalid", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsRatioNumber).Return(faker.Word(), nil).Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		assert.ErrorIs(t, err, ErrInvalid)
		assert.Nil(t, actual)
	})

	t.Run("err include err not found", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsRatioNumber).Return(fmt.Sprintf("%d", expectedPct), nil).Once()
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsPersistErrors).Return("", assert.AnError).Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		assert.ErrorIs(t, err, ErrNotFound)
		assert.Nil(t, actual)
	})

	t.Run("err include err invalid", func(t *testing.T) {
		t.Parallel()

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsRatioNumber).Return(fmt.Sprintf("%d", expectedPct), nil).Once()
		mockAccessor.EXPECT().GetVariable(namespace, connection, varLogsPersistErrors).Return(fmt.Sprintf("%d", expectedPct), nil).Once()

		target := NewMDAISettingController(c, mockAccessor, nil)

		actual, err := target.GetFilter(budgetv1alpha.FilterType_FILTER_TYPE_LOG, namespace, connection)
		assert.ErrorIs(t, err, ErrInvalid)
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

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varLogsRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varLogsPersistErrors, mock.Anything).Return(nil).Once()
		mockKube := kubernetesmock.NewMockInterface(t)
		mockAppsv1 := kubeappsv1mock.NewMockAppsV1Interface(t)
		mockDeployment := kubeappsv1mock.NewMockDeploymentInterface(t)
		mockKube.EXPECT().AppsV1().Return(mockAppsv1)
		mockAppsv1.EXPECT().Deployments(namespace).Return(mockDeployment)
		mockDeployment.EXPECT().Get(mock.Anything, controllerName, mock.Anything).Return(&v1.Deployment{
			Status: v1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			},
		}, nil)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		err := target.UpdateFilter(t.Context(), namespace, connection, &input)
		assert.NoError(t, err)
	})

	t.Run("success trace", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
			PctSampled: 10,
			IncludeErr: true,
		}

		controllerName := fmt.Sprintf(collectorTraceNameFormatter, connection)

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesPersistErrors, mock.Anything).Return(nil).Once()
		mockKube := kubernetesmock.NewMockInterface(t)
		mockAppsv1 := kubeappsv1mock.NewMockAppsV1Interface(t)
		mockDeployment := kubeappsv1mock.NewMockDeploymentInterface(t)
		mockKube.EXPECT().AppsV1().Return(mockAppsv1)
		mockAppsv1.EXPECT().Deployments(namespace).Return(mockDeployment)
		mockDeployment.EXPECT().Get(mock.Anything, controllerName, mock.Anything).Return(&v1.Deployment{
			Status: v1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			},
		}, nil)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		err := target.UpdateFilter(t.Context(), namespace, connection, &input)
		assert.NoError(t, err)
	})

	t.Run("err log lock in use", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_LOG,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)
		target.log.Lock()

		err := target.UpdateFilter(t.Context(), namespace, connection, &input)
		assert.ErrorIs(t, err, ErrStillUpdating)
	})

	t.Run("err trace lock in use", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)
		target.trace.Lock()

		err := target.UpdateFilter(t.Context(), namespace, connection, &input)
		assert.ErrorIs(t, err, ErrStillUpdating)
	})

	t.Run("err update ratio num", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_LOG,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varLogsRatioNumber, mock.Anything).Return(assert.AnError).Once()
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		err := target.UpdateFilter(t.Context(), namespace, connection, &input)
		assert.Error(t, err)
	})

	t.Run("err update include err", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_LOG,
			PctSampled: 10,
			IncludeErr: true,
		}

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varLogsRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varLogsPersistErrors, mock.Anything).Return(assert.AnError).Once()
		mockKube := kubernetesmock.NewMockInterface(t)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		err := target.UpdateFilter(t.Context(), namespace, connection, &input)
		assert.Error(t, err)
	})

	t.Run("err get deployment", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
			PctSampled: 10,
			IncludeErr: true,
		}

		controllerName := fmt.Sprintf(collectorTraceNameFormatter, connection)

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesPersistErrors, mock.Anything).Return(nil).Once()
		mockKube := kubernetesmock.NewMockInterface(t)
		mockAppsv1 := kubeappsv1mock.NewMockAppsV1Interface(t)
		mockDeployment := kubeappsv1mock.NewMockDeploymentInterface(t)
		mockKube.EXPECT().AppsV1().Return(mockAppsv1)
		mockAppsv1.EXPECT().Deployments(namespace).Return(mockDeployment)
		mockDeployment.EXPECT().Get(mock.Anything, controllerName, mock.Anything).Return(nil, assert.AnError).Once()

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		err := target.UpdateFilter(t.Context(), namespace, connection, &input)
		assert.Error(t, err)
	})

	t.Run("err timeout", func(t *testing.T) {
		t.Parallel()

		input := budgetv1alpha.Filter{
			Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
			PctSampled: 10,
			IncludeErr: true,
		}

		controllerName := fmt.Sprintf(collectorTraceNameFormatter, connection)

		mockAccessor := budgetfiltermock.NewMockVariableAccessor(t)
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesRatioNumber, mock.Anything).Return(nil).Once()
		mockAccessor.EXPECT().UpdateVariable(namespace, connection, varTracesPersistErrors, mock.Anything).Return(nil).Once()
		mockKube := kubernetesmock.NewMockInterface(t)
		mockAppsv1 := kubeappsv1mock.NewMockAppsV1Interface(t)
		mockDeployment := kubeappsv1mock.NewMockDeploymentInterface(t)
		mockKube.EXPECT().AppsV1().Return(mockAppsv1)
		mockAppsv1.EXPECT().Deployments(namespace).Return(mockDeployment)
		mockDeployment.EXPECT().Get(mock.Anything, controllerName, mock.Anything).Return(&v1.Deployment{
			Status: v1.DeploymentStatus{
				Replicas:      2,
				ReadyReplicas: 1,
			},
		}, nil)

		target := NewMDAISettingController(c, mockAccessor, mockKube)

		err := target.UpdateFilter(t.Context(), namespace, connection, &input)
		assert.ErrorIs(t, err, ErrTimeout)
	})
}
