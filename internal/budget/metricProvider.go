package budget

import (
	"context"
	"fmt"
	"strconv"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
	"github.com/mydecisive/octant/internal/config"
)

const (
	pctConstant = 100
)

// MetricDataProvider is used to retrieve metric data.
type MetricDataProvider interface {
	// GetOverall returns the overall summary of the log and span data for the given timeframe.
	GetOverall(
		ctx context.Context,
		timeframe budgetv1alpha.Timeframe,
		namespace string,
	) (*budgetv1alpha.Overall, error)
	// GetLogs returns the list of log data that matches the given input.
	GetLogs(ctx context.Context, input budgetdata.MetricDataInput) ([]*budgetv1alpha.Log, string, error)
	// GetSpans returns the list of span data that matches the given input.
	GetSpans(ctx context.Context, input budgetdata.MetricDataInput) ([]*budgetv1alpha.Span, string, error)
}

type MetricProvider struct {
	config    *config.Configuration
	retriever budgetdata.MetricDataRetriever
}

// Ensure MetricProvider implements MetricDataProvider.
var _ MetricDataProvider = &MetricProvider{}

// NewMetricProvider returns a new instance of MetricProvider.
func NewMetricProvider(c *config.Configuration, retriever budgetdata.MetricDataRetriever) *MetricProvider {
	return &MetricProvider{
		config:    c,
		retriever: retriever,
	}
}

// GetOverall retrieves basic summary metrics from data store and then perform
// additional calculations base on the metrics to generated the overall summary.
func (mp *MetricProvider) GetOverall(
	ctx context.Context,
	timeframe budgetv1alpha.Timeframe,
	namespace string,
) (*budgetv1alpha.Overall, error) {
	raw, err := mp.retriever.GetOverall(ctx, timeframe, namespace)
	if err != nil {
		return nil, err
	}

	logCost, err := mp.logCost(raw.LogSend)
	if err != nil {
		return nil, err
	}
	traceCost, err := mp.traceCost(raw.SpanSend)
	if err != nil {
		return nil, err
	}
	totalCost := logCost + traceCost

	logPct, err := mp.pct(logCost, totalCost)
	if err != nil {
		return nil, err
	}

	return &budgetv1alpha.Overall{
		Cost: totalCost,
		Log: &budgetv1alpha.Overall_Metric{
			Received: raw.LogReceived,
			Sent:     raw.LogSend,
			Filtered: raw.LogReceived - raw.LogSend,
			CostRate: mp.config.Budget.DefaultLogCostRate,
			Cost:     logCost,
			Pct:      float32(logPct),
		},
		Trace: &budgetv1alpha.Overall_Metric{
			Received: raw.SpanReceived,
			Sent:     raw.SpanSend,
			Filtered: raw.SpanReceived - raw.SpanSend,
			CostRate: mp.config.Budget.DefaultTraceCostRate,
			Cost:     traceCost,
			Pct:      float32(float64(pctConstant) - logPct),
		},
	}, nil
}

// GetLogs retrieves the log metric from the data store and then
// perform additional calculations base on the log metrics to generate the log budget data.
func (mp *MetricProvider) GetLogs(
	ctx context.Context,
	input budgetdata.MetricDataInput,
) ([]*budgetv1alpha.Log, string, error) {
	total, err := mp.retriever.GetTotalLog(ctx, input.Timeframe, input.Namespace)
	if err != nil {
		return nil, "", err
	}
	raw, nextPage, err := mp.retriever.GetLogs(ctx, input)
	if err != nil {
		return nil, "", err
	}

	result := make([]*budgetv1alpha.Log, len(raw))
	for i, rlog := range raw {
		cost, err := mp.logCost(rlog.Amount)
		if err != nil {
			return nil, "", err
		}

		pct, err := mp.pct(float64(rlog.Amount), float64(total))
		if err != nil {
			return nil, "", err
		}

		result[i] = &budgetv1alpha.Log{
			Name: rlog.Name,
			Sent: rlog.Amount,
			Pct:  float32(pct),
			Cost: cost,
		}
	}
	return result, nextPage, nil
}

// GetSpans retrieves the span metric from the data store and then
// perform additional calculations base on the span metrics to generate the span budget data.
func (mp *MetricProvider) GetSpans(
	ctx context.Context,
	input budgetdata.MetricDataInput,
) ([]*budgetv1alpha.Span, string, error) {
	raw, nextPage, err := mp.retriever.GetRootSpans(ctx, input)
	if err != nil {
		return nil, "", err
	}

	result := make([]*budgetv1alpha.Span, len(raw))
	for i, r := range raw {
		cost, err := mp.traceCost(r.Count)
		if err != nil {
			return nil, "", err
		}

		result[i] = &budgetv1alpha.Span{
			Name:        r.Name,
			Breath:      r.Breath,
			Depth:       r.Depth,
			Invocations: r.Invocation,
			Cost:        cost,
		}
	}
	return result, nextPage, nil
}

// traceCost returns the trace cost calculated base on the given count.
func (mp *MetricProvider) traceCost(count int64) (float64, error) {
	return mp.truncate(
		float64(count) * float64(mp.config.Budget.DefaultTraceCostRate),
	)
}

// logCost returns the log cost calculated base on the given log data amount.
func (mp *MetricProvider) logCost(amount int64) (float64, error) {
	return mp.truncate(float64(amount) * float64(mp.config.Budget.DefaultLogCostRate))
}

// pct calculates the percentage base on a/b
// and then returns the formatted percentage number.
func (mp *MetricProvider) pct(a, b float64) (float64, error) {
     if b == 0 {
        return 0
    }
	return mp.truncate((a / b) * pctConstant)
}

// truncate truncates the given float to the nearest 2 decimal place.
func (*MetricProvider) truncate(cost float64) (float64, error) {
	return strconv.ParseFloat(fmt.Sprintf("%.2f", cost), 64)
}
