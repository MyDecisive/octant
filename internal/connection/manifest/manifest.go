package manifest

import (
	"context"
	"errors"
	"fmt"
	"maps"

	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
)

var (
	ErrGetTemplate    = errors.New("get template")
	ErrParseTemplate  = errors.New("parse template")
	ErrRenderTemplate = errors.New("render template")
	ErrConvertJSON    = errors.New("convert json")
	ErrPushApp        = errors.New("push app")
	ErrPushManifests  = errors.New("push manifests")
	ErrEmpty          = errors.New("empty")
)

// Generator generates manifest(s).
type Generator interface {
	// All returns manifests for all apps, connections, and validators.
	All(ctx context.Context, input manifestdata.AllInput, format manifestdata.OutputFormat) (map[string][]byte, error)
	// App returns manifest for the given App template type in the provided format using the data.
	App(app manifestdata.App, data manifestdata.AppTemplateData, format manifestdata.OutputFormat) ([][]byte, error)
	// Connections returns manifest for all connection template files in the provided format using the input.
	// Note: the map key will be the file name and the map value is the file content.
	Connections(
		ctx context.Context,
		input manifestdata.ConnectionInput,
		format manifestdata.OutputFormat,
	) (map[string][]byte, error)
	// Validators returns manifest for all validator template files in the provided format using the input.
	// Note: the map key will be the file name and the map value is the file content.
	Validators(input manifestdata.ValidatorInput, format manifestdata.OutputFormat) (map[string][]byte, error)
}

// ManifestGenerator implements Generator.
type ManifestGenerator struct {
	provider TemplateProvider
	renderer TemplateRenderer
	mapper   manifestdata.Mapper
}

// Ensure ManifestGenerator implements Generator.
var _ Generator = (*ManifestGenerator)(nil)

// NewManifestGenerator returns a new instance of ManifestGenerator.
func NewManifestGenerator(
	provider TemplateProvider,
	renderer TemplateRenderer,
	mapper manifestdata.Mapper,
) *ManifestGenerator {
	return &ManifestGenerator{
		provider: provider,
		renderer: renderer,
		mapper:   mapper,
	}
}

// All returns manifests for all apps, connections, and validators.
func (mg *ManifestGenerator) All(
	ctx context.Context,
	input manifestdata.AllInput,
	format manifestdata.OutputFormat,
) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, app := range manifestdata.AppValues() {
		name := app.String()
		data := mg.mapper.AppTemplateData(
			app,
			input.MDAIVersion,
			input.ConnectionName,
			input.Namespace,
		)
		contents, err := mg.App(app, data, format)
		if err != nil {
			return nil, fmt.Errorf("%s:%w", name, err)
		}
		for i, content := range contents {
			result[mg.getFilename(fmt.Sprintf("%s%d", name, i), format)] = content
		}
	}

	conn, err := mg.Connections(ctx, manifestdata.ConnectionInput{
		ConnectionName: input.ConnectionName,
		Namespace:      input.Namespace,
		TelemetryTypes: input.TelemetryTypes,
		Exported:       input.Exported,
	}, format)
	if err != nil {
		return nil, fmt.Errorf("connection templates:%w", err)
	}
	maps.Copy(result, conn)

	validator, err := mg.Validators(manifestdata.ValidatorInput{
		ConnectionName: input.ConnectionName,
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
func (mg *ManifestGenerator) App(
	app manifestdata.App,
	data manifestdata.AppTemplateData,
	format manifestdata.OutputFormat,
) ([][]byte, error) {
	raw, err := mg.provider.GetApp(app)
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrGetTemplate, err)
	}

	return mg.renderer.Render(app.String(), raw, format, data)
}

// Connections returns manifest for all connection template files in the provided format using the data.
// Note: the map key will be the file name and the map value is the file content.
func (mg *ManifestGenerator) Connections(
	ctx context.Context,
	input manifestdata.ConnectionInput,
	format manifestdata.OutputFormat,
) (map[string][]byte, error) {
	templates, err := mg.provider.GetAllConnections()
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrGetTemplate, err)
	}

	data, err := mg.mapper.ConnectionTemplateData(ctx, input)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for conn, raw := range templates {
		name := conn.String()
		manifests, err := mg.renderer.Render(name, raw, format, data)
		if err != nil {
			return nil, err
		}
		for i, manifest := range manifests {
			result[mg.getFilename(fmt.Sprintf("%s%d", name, i), format)] = manifest
		}
	}

	return result, nil
}

// Validators returns manifest for all validator template files in the provided format using the data.
// Note: the map key will be the file name and the map value is the file content.
func (mg *ManifestGenerator) Validators(
	input manifestdata.ValidatorInput,
	format manifestdata.OutputFormat,
) (map[string][]byte, error) {
	templates, err := mg.provider.GetAllValidators()
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrGetTemplate, err)
	}
	data := mg.mapper.ValidatorTemplateData(input)

	result := make(map[string][]byte)
	for conn, raw := range templates {
		name := conn.String()
		manifests, err := mg.renderer.Render(name, raw, format, data)
		if err != nil {
			return nil, err
		}
		for i, manifest := range manifests {
			result[mg.getFilename(fmt.Sprintf("%s%d", name, i), format)] = manifest
		}
	}

	return result, nil
}

// getFilename returns the file name base on given format.
func (*ManifestGenerator) getFilename(name string, outputFormat manifestdata.OutputFormat) string {
	return fmt.Sprintf("%s.%s", name, outputFormat.String())
}
