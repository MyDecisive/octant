package manifest

import (
	"errors"
	"fmt"
	"maps"

	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/telemetry"
)

var (
	ErrGetTemplate    = errors.New("get template")
	ErrParseTemplate  = errors.New("parse template")
	ErrRenderTemplate = errors.New("render template")
	ErrConvertJSON    = errors.New("convert json")
)

// AllInput is the input for `All` method of the generator.
type AllInput struct {
	IsArgoSideload         bool
	ConnectionName         string
	Namespace              string // MDAI namespace
	TelemetryTypes         []telemetry.MLT
	DatadogIntegrationData *integration.DataDogIntegrationData
	ValidatorRunID         string
	MDAIVersion            string
}

// ConnectionInput is the input for `Connections` method of the generator.
type ConnectionInput struct {
	IsArgoSideload         bool
	Name                   string // connection name
	Namespace              string
	TelemetryTypes         []telemetry.MLT
	DatadogIntegrationData *integration.DataDogIntegrationData
}

// ValidatorInput is the input for `Validators` method of the generator.
type ValidatorInput struct {
	IsArgoSideload bool
	Name           string // connection name
	Namespace      string
	RunID          string
}

// Generator generates manifest(s).
type Generator interface {
	// All returns manifests for all apps, connections, and validators.
	All(input AllInput, format OutputFormat) (map[string][]byte, error)
	// App returns manifest for the given App template type in the provided format using the data.
	App(app App, data AppTemplateData, format OutputFormat) ([]byte, error)
	// Connections returns manifest for all connection template files in the provided format using the input.
	// Note: the map key will be the file name and the map value is the file content.
	Connections(input ConnectionInput, format OutputFormat) (map[string][]byte, error)
	// Validators returns manifest for all validator template files in the provided format using the input.
	// Note: the map key will be the file name and the map value is the file content.
	Validators(input ValidatorInput, format OutputFormat) (map[string][]byte, error)
}

// ManifestGenerator implements Generator.
type ManifestGenerator struct {
	provider TemplateProvider
	renderer TemplateRenderer
	config   *config.Configuration
}

// Ensure ManifestGenerator implements Generator.
var _ Generator = (*ManifestGenerator)(nil)

func (mg *ManifestGenerator) All(input AllInput, format OutputFormat) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, app := range AppValues() {
		name := app.String()
		data := GetAppTemplateData(
			app,
			mg.config,
			input.MDAIVersion,
			input.ConnectionName,
			input.Namespace,
		)
		content, err := mg.App(app, data, format)
		if err != nil {
			return nil, fmt.Errorf("%s:%w", name, err)
		}
		result[mg.getFilename(name, format)] = content
	}

	conn, err := mg.Connections(ConnectionInput{
		IsArgoSideload:         input.IsArgoSideload,
		Name:                   input.ConnectionName,
		Namespace:              input.Namespace,
		TelemetryTypes:         input.TelemetryTypes,
		DatadogIntegrationData: input.DatadogIntegrationData,
	}, format)
	if err != nil {
		return nil, fmt.Errorf("connection templates:%w", err)
	}
	maps.Copy(result, conn)

	validator, err := mg.Validators(ValidatorInput{
		IsArgoSideload: input.IsArgoSideload,
		Name:           input.ConnectionName,
		Namespace:      input.Namespace,
		RunID:          input.ValidatorRunID,
	}, format)
	if err != nil {
		return nil, fmt.Errorf("validator templates:%w", err)
	}
	maps.Copy(result, validator)
	return result, nil
}

// App returns manifest for the given App template type in the provided format using the data.
func (mg *ManifestGenerator) App(app App, data AppTemplateData, format OutputFormat) ([]byte, error) {
	raw, err := mg.provider.GetApp(app)
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrGetTemplate, err)
	}

	return mg.renderer.Render(app.String(), raw, format, data)
}

// Connections returns manifest for all connection template files in the provided format using the data.
// Note: the map key will be the file name and the map value is the file content.
func (mg *ManifestGenerator) Connections(
	input ConnectionInput,
	format OutputFormat,
) (map[string][]byte, error) {
	templates, err := mg.provider.GetAllConnections()
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrGetTemplate, err)
	}
	data := getConnectionTemplateData(mg.config, input)

	result := make(map[string][]byte)
	for conn, raw := range templates {
		name := conn.String()
		manifest, err := mg.renderer.Render(name, raw, format, data)
		if err != nil {
			return nil, err
		}
		result[mg.getFilename(name, format)] = manifest
	}

	return result, nil
}

// Validators returns manifest for all validator template files in the provided format using the data.
// Note: the map key will be the file name and the map value is the file content.
func (mg *ManifestGenerator) Validators(
	input ValidatorInput,
	format OutputFormat,
) (map[string][]byte, error) {
	templates, err := mg.provider.GetAllValidators()
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrGetTemplate, err)
	}
	data := getValidatorTemplateData(mg.config, input)

	result := make(map[string][]byte)
	for conn, raw := range templates {
		name := conn.String()
		manifest, err := mg.renderer.Render(name, raw, format, data)
		if err != nil {
			return nil, err
		}
		result[mg.getFilename(name, format)] = manifest
	}

	return result, nil
}

// getFilename returns the file name base on given format.
func (*ManifestGenerator) getFilename(name string, outputFormat OutputFormat) string {
	return fmt.Sprintf("%s.%s", name, outputFormat)
}
