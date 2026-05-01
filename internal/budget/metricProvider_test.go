package budget

import (
	"fmt"
	"strconv"
	"testing"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/go-faker/faker/v4"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
	"github.com/mydecisive/octant/internal/config"
	budgetdatamock "github.com/mydecisive/octant/internal/mock/budgetdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMetricProvider_GetOverall(t *testing.T) {
	t.Parallel()

	timeframe := budgetv1alpha.Timeframe_TIMEFRAME_LM
	namespace := faker.Word()

	c := &config.Configuration{
		Budget: config.Budget{
			DefaultLogCostRate:   2.5,
			DefaultTraceCostRate: 2,
		},
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		raw := budgetdata.Overall{
			LogReceived:  20,
			LogSend:      10,
			SpanReceived: 20,
			SpanSend:     10,
		}

		expected := &budgetv1alpha.Overall{
			Log: &budgetv1alpha.Overall_Metric{
				Received: raw.LogReceived,
				Sent:     raw.LogSend,
				Filtered: raw.LogReceived - raw.LogSend,
				CostRate: c.Budget.DefaultLogCostRate,
				Cost:     float64(raw.LogSend) * float64(c.Budget.DefaultLogCostRate),
				Pct:      55.56,
			},
			Trace: &budgetv1alpha.Overall_Metric{
				Received: raw.SpanReceived,
				Sent:     raw.SpanSend,
				Filtered: raw.SpanReceived - raw.SpanSend,
				CostRate: c.Budget.DefaultTraceCostRate,
				Cost:     float64(raw.SpanSend) * float64(c.Budget.DefaultTraceCostRate),
				Pct:      44.44,
			},
		}
		expected.Cost = expected.GetLog().GetCost() + expected.GetTrace().GetCost()
		logPct, err := strconv.ParseFloat(fmt.Sprintf("%.2f", (expected.GetLog().GetCost()/expected.GetCost())*100), 64)
		require.NoError(t, err)
		expected.Log.Pct = float32(logPct)
		expected.Trace.Pct = float32(100) - float32(logPct)

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetOverall(mock.Anything, timeframe, namespace).Return(&raw, nil).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetOverall(t.Context(), timeframe, namespace)
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetOverall(mock.Anything, timeframe, namespace).Return(nil, assert.AnError).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetOverall(t.Context(), timeframe, namespace)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})
}

func TestMetricProvider_GetLogs(t *testing.T) {
	t.Parallel()

	timeframe := budgetv1alpha.Timeframe_TIMEFRAME_LM
	namespace := faker.Word()

	c := &config.Configuration{
		Budget: config.Budget{
			DefaultLogCostRate:   2.5,
			DefaultTraceCostRate: 2,
		},
	}
	total := 20

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		input := budgetdata.MetricDataInput{
			Timeframe: timeframe,
			Namespace: namespace,
		}

		raw := budgetdata.Log{
			Name:   faker.Word(),
			Amount: 10,
		}

		expected := &budgetv1alpha.Log{
			Name: raw.Name,
			Sent: raw.Amount,
			Cost: float64(raw.Amount) * float64(c.Budget.DefaultLogCostRate),
		}
		pct, err := strconv.ParseFloat(fmt.Sprintf("%.2f", (float64(raw.Amount)/float64(total))*100), 64)
		require.NoError(t, err)
		expected.Pct = float32(pct)

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetTotalLog(mock.Anything, timeframe, namespace).Return(int64(total), nil).Once()
		mockRetriever.EXPECT().GetLogs(mock.Anything, input).Return([]budgetdata.Log{raw}, nil).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetLogs(t.Context(), input)
		require.NoError(t, err)

		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success no logs", func(t *testing.T) {
		t.Parallel()

		input := budgetdata.MetricDataInput{
			Timeframe: timeframe,
			Namespace: namespace,
		}

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetTotalLog(mock.Anything, timeframe, namespace).Return(int64(total), nil).Once()
		mockRetriever.EXPECT().GetLogs(mock.Anything, input).Return([]budgetdata.Log{}, nil).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetLogs(t.Context(), input)
		require.NoError(t, err)

		assert.Empty(t, actual)
	})

	t.Run("err total log", func(t *testing.T) {
		t.Parallel()

		input := budgetdata.MetricDataInput{
			Timeframe: timeframe,
			Namespace: namespace,
		}

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetTotalLog(mock.Anything, timeframe, namespace).Return(0, assert.AnError).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetLogs(t.Context(), input)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})

	t.Run("err get logs", func(t *testing.T) {
		t.Parallel()

		input := budgetdata.MetricDataInput{
			Timeframe: timeframe,
			Namespace: namespace,
		}

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetTotalLog(mock.Anything, timeframe, namespace).Return(int64(total), nil).Once()
		mockRetriever.EXPECT().GetLogs(mock.Anything, input).Return(nil, assert.AnError).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetLogs(t.Context(), input)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})
}

func TestMetricProvider_GetSpans(t *testing.T) {
	t.Parallel()

	timeframe := budgetv1alpha.Timeframe_TIMEFRAME_LM
	namespace := faker.Word()

	c := &config.Configuration{
		Budget: config.Budget{
			DefaultLogCostRate:   2.5,
			DefaultTraceCostRate: 2,
		},
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		input := budgetdata.MetricDataInput{
			Timeframe: timeframe,
			Namespace: namespace,
		}

		var raw budgetdata.RootSpan
		require.NoError(t, faker.FakeData(&raw))
		raw.Count = 10

		expected := &budgetv1alpha.Span{
			Name:        raw.Name,
			Breath:      raw.Breath,
			Depth:       raw.Depth,
			Invocations: raw.Invocation,
			Cost:        float64(raw.Count) * float64(c.Budget.DefaultTraceCostRate),
		}

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetRootSpans(mock.Anything, input).Return([]budgetdata.RootSpan{raw}, nil).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetSpans(t.Context(), input)
		require.NoError(t, err)

		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success no data", func(t *testing.T) {
		t.Parallel()

		input := budgetdata.MetricDataInput{
			Timeframe: timeframe,
			Namespace: namespace,
		}

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetRootSpans(mock.Anything, input).Return([]budgetdata.RootSpan{}, nil).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetSpans(t.Context(), input)
		require.NoError(t, err)

		assert.Empty(t, actual)
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		input := budgetdata.MetricDataInput{
			Timeframe: timeframe,
			Namespace: namespace,
		}

		mockRetriever := budgetdatamock.NewMockMetricDataRetriever(t)
		mockRetriever.EXPECT().GetRootSpans(mock.Anything, input).Return(nil, assert.AnError).Once()

		target := NewMetricProvider(c, mockRetriever)
		actual, err := target.GetSpans(t.Context(), input)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})
}
