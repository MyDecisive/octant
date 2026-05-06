package budget

import (
	"context"

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

	result := &budgetv1alpha.Overall{
		Log: &budgetv1alpha.Overall_Metric{
			Received: raw.LogReceived,
			Sent:     raw.LogSend,
			Filtered: raw.LogReceived - raw.LogSend,
			CostRate: float32(mp.config.Budget.DefaultLogCostRate),
			Cost:     mp.logCost(raw.LogSend),
		},
		Trace: &budgetv1alpha.Overall_Metric{
			Received: raw.SpanReceived,
			Sent:     raw.SpanSend,
			Filtered: raw.SpanReceived - raw.SpanSend,
			CostRate: float32(mp.config.Budget.DefaultTraceCostRate),
			Cost:     mp.traceCost(raw.SpanSend),
		},
	}
	result.Cost = result.GetLog().GetCost() + result.GetTrace().GetCost()
	result.Log.Pct = float32(mp.pct(result.GetLog().GetCost(), result.GetCost()))
	if result.GetTrace().GetCost() > 0 {
		result.Trace.Pct = float32(pctConstant - result.GetLog().GetPct())
	}

	if result.GetLog().GetFiltered() < 0 {
		result.Log.Filtered = 0
	}
	if result.GetTrace().GetFiltered() < 0 {
		result.Trace.Filtered = 0
	}

	return result, nil
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
	if total <= 0 {
		return []*budgetv1alpha.Log{}, "", nil
	}

	raw, nextPage, err := mp.retriever.GetLogs(ctx, input)
	if err != nil {
		return nil, "", err
	}

	result := make([]*budgetv1alpha.Log, len(raw))
	for i, rlog := range raw {
		result[i] = &budgetv1alpha.Log{
			Name: rlog.Name,
			Sent: rlog.Amount,
			Pct:  float32(mp.pct(rlog.Amount, total)),
			Cost: mp.logCost(rlog.Amount),
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
		result[i] = &budgetv1alpha.Span{
			Name:        r.Name,
			Breadth:     r.Breadth,
			Depth:       r.Depth,
			Invocations: r.Invocation,
			Cost:        mp.traceCost(r.Count),
		}
	}
	return result, nextPage, nil
}

// traceCost returns the trace cost calculated base on the given count.
func (mp *MetricProvider) traceCost(count float64) float64 {
	return count * mp.config.Budget.DefaultTraceCostRate
}

// logCost returns the log cost calculated base on the given log data amount.
func (mp *MetricProvider) logCost(amount float64) float64 {
	return amount * mp.config.Budget.DefaultLogCostRate
}

// pct calculates the percentage base on a/b
// and then returns the formatted percentage number.
func (*MetricProvider) pct(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return (a / b) * pctConstant
}
