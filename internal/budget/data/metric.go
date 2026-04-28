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
	Name     string
	Received uint64
}

// MetricDataRetrieverInput contains parameters needed by the MetricDataRetriever methods.
type MetricDataRetrieverInput struct {
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
	// GetLogs returns the list of log data that matches the given inputs.
	GetLogs(input MetricDataRetrieverInput) ([]Log, error)
}
