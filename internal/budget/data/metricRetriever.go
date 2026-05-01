package budgetdata

import (
	"context"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
)

// Overall represents an overall metric data entry from the data store.
type Overall struct {
	LogReceived  int64 // in GB
	LogSend      int64 // in GB
	SpanReceived int64 // in million events
	SpanSend     int64 // in million events
}

// Log represents a single log data entry from the data store.
type Log struct {
	Name   string
	Amount int64 // Send amount in GB
}

// RootSpan represents a single root span data entry from the data store.
type RootSpan struct {
	Name       string
	Breath     uint32
	Depth      uint32
	Invocation uint32
	Count      int64 // Send count in million events
}

// MetricDataInput contains parameters needed to retrieve metric data.
type MetricDataInput struct {
	Timeframe budgetv1alpha.Timeframe
	Size      uint32
	PageToken string
	Search    string
	Namespace string
}

// MetricDataRetriever is used to retrieve metric data from the data store.
type MetricDataRetriever interface {
	// GetOverall returns the overall summary of the log and span data for the given timeframe.
	GetOverall(ctx context.Context, timeframe budgetv1alpha.Timeframe, namespace string) (*Overall, error)
	// GetTotalLog returns total amount of log data sent.
	GetTotalLog(ctx context.Context, timeframe budgetv1alpha.Timeframe, namespace string) (int64, error)
	// GetLogs returns the list of log data that matches the given inputs.
	GetLogs(ctx context.Context, input MetricDataInput) ([]Log, string, error)
	// GetRootSpans returns the list of root span data that matches the given inputs.
	GetRootSpans(ctx context.Context, input MetricDataInput) ([]RootSpan, string, error)
}
