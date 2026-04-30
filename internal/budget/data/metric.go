package budgetdata

import (
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
)

// Overall represents an overall metric data entry from the data store.
type Overall struct {
	LogReceived  uint64
	LogSend      uint64
	SpanReceived uint64
	SpanSend     uint64
}

// Log represents a single log data entry from the data store.
type Log struct {
	Name string
	Sent uint64
}

// RootSpan represents a single root span data entry from the data store.
type RootSpan struct {
	Name       string
	Breath     uint32
	Depth      uint32
	Invocation uint32
	Count      uint64
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
	GetOverall(timeframe budgetv1alpha.Timeframe, namespace string) (*Overall, error)
	// GetTotalLog returns total amount of log data sent.
	GetTotalLog(timeframe budgetv1alpha.Timeframe, namespace string) (uint64, error)
	// GetLogs returns the list of log data that matches the given inputs.
	GetLogs(input MetricDataInput) ([]Log, error)
	// GetRootSpans returns the list of root span data that matches the given inputs.
	GetRootSpans(input MetricDataInput) ([]RootSpan, error)
}
