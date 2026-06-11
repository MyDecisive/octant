package manifestdata

import (
	"github.com/mydecisive/octant/internal/telemetry"
)

// AllInput is the input for `All` method of the generator.
type AllInput struct {
	ConnectionName string
	Namespace      string // MDAI namespace
	TelemetryTypes []telemetry.MLT
	ValidatorRunID string
	MDAIVersion    string
	Exported       bool
}

type Destination struct {
	Type            DestinationType
	IntegrationName string
}

// ConnectionInput is the input for `Connections` method of the generator.
type ConnectionInput struct {
	ConnectionName            string
	DeploymentIntegrationName string // only used by manifest manager
	Namespace                 string
	TelemetryTypes            []telemetry.MLT
	Destinations              []Destination
	Exported                  bool
}

// ValidatorInput is the input for `Validators` method of the generator.
type ValidatorInput struct {
	ConnectionName            string
	DeploymentIntegrationName string // only used by manifest manager
	Namespace                 string
	RunID                     string
}
