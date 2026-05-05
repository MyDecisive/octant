package budgetdata

import budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"

// Overall represents an overall metric data entry from the data store.
type Overall struct {
	LogReceived  float64 // in GB
	LogSend      float64 // in GB
	SpanReceived float64 // in million events
	SpanSend     float64 // in million events
}

// Log represents a single log data entry from the data store.
type Log struct {
	Name   string
	Amount float64 // Send amount in GB
}

// RootSpan represents a single root span data entry from the data store.
type RootSpan struct {
	Name       string
	Breadth    float64
	Depth      float64
	Invocation float64
	Count      float64 // Send count in million events
}

// MetricDataInput contains parameters needed to retrieve metric data.
type MetricDataInput struct {
	Timeframe budgetv1alpha.Timeframe
	Size      uint32
	PageToken string
	Search    string
	Namespace string
}
