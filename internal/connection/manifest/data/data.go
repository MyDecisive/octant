package manifestdata

import (
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/telemetry"
)

// AppTemplateData will be passed to the renderer as the data input
// for argocd app template.
type AppTemplateData struct {
	Name string // connection name
	// Not applicable for Validator and Connection.
	// For Cert-Manager, this value comes from config.
	// For MDAI, this value provided by user.
	Version string
	// For Cert-Manager, this value comes from config.
	Namespace string
}

// ConnectionTemplateData will be passed to the renderer as the data input
// for connection manifest template.
type ConnectionTemplateData struct {
	Name                   string // connection name
	Namespace              string
	CurrentNamespace       string // from config
	TelemetryTypes         []telemetry.MLT
	DatadogIntegrationData *integration.DataDogIntegrationData
	ServiceAccount         string // from config
	DefaultLogRatio        string // from config
	DefaultLogPersistErr   bool   // from config
	DefaultTraceRatio      string // from config
	DefaultTracePersistErr bool   // from config
}

// ValidatorTemplateData will be passed to the renderer as the data input
// for validator manifest template.
type ValidatorTemplateData struct {
	Name      string // connection name
	Namespace string
	Version   string // from config
	RunID     string
}
