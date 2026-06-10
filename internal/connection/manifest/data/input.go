package manfiestdata

import (
	"github.com/mydecisive/octant/internal/telemetry"
)

// AllInput is the input for `All` method of the generator.
type AllInput struct {
	IsArgoSideload bool
	ConnectionName string
	Namespace      string // MDAI namespace
	TelemetryTypes []telemetry.MLT
	ValidatorRunID string
	MDAIVersion    string
}

// ConnectionInput is the input for `Connections` method of the generator.
type ConnectionInput struct {
	IsArgoSideload bool
	Name           string // connection name
	Namespace      string
	TelemetryTypes []telemetry.MLT
	Destinations   []Destination
	Dummy          bool
}

// ValidatorInput is the input for `Validators` method of the generator.
type ValidatorInput struct {
	IsArgoSideload bool
	Name           string // connection name
	Namespace      string
	RunID          string
}
