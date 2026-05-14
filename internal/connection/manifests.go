package connection

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"text/template"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/metrics"
	"sigs.k8s.io/yaml"
)

//go:embed manifests/cert-manager.yaml
var CertManagerAppManifest []byte

//go:embed templates/argo-app.yaml.tmpl
var argoAppTemplate string

//go:embed templates/lb-collector.yaml.tmpl
var lbCollectorTemplate string

//go:embed templates/log-collector.yaml.tmpl
var logCollectorTemplate string

//go:embed templates/trace-collector.yaml.tmpl
var traceCollectorTemplate string

//go:embed templates/mdai-app.yaml.tmpl
var mdaiAppTemplate string

//go:embed templates/hub.yaml.tmpl
var hubTemplate string

//go:embed templates/observer.yaml.tmpl
var observerTemplate string

//go:embed templates/validator.yaml.tmpl
var validatorTemplate string

//go:embed templates/secret.yaml.tmpl
var secretTemplate string

//go:embed templates/additional.yaml.tmpl
var additionalTemplate string

type ArgoConnectionTemplateData struct {
	AppName                string
	Namespace              string
	ServiceAccount         string
	CurrentNamespace       string
	ConnectionData         OctantConnectionData
	DatadogIntegrationData *integration.DataDogIntegrationData
	IsArgoSideload         bool
}

type ArgoValidatorTemplateData struct {
	ConnectionName string
	Namespace      string
	ValidatorRunID string
}

type ManifestOutputFormat string

const (
	YAMLOutputFormat ManifestOutputFormat = "yaml"
	JSONOutputFormat ManifestOutputFormat = "json"
)

func getRunID() string {
	return time.Now().UTC().Format(metrics.ValidatorRunIDFormat)
}

func (oc *OctantConnection) createTemplateData(
	ctx context.Context,
	name string,
	connection OctantConnectionData,
) (*ArgoConnectionTemplateData, error) {
	if len(connection.Destinations) > 1 {
		// TODO: Implement multiple destination handling and handling of non-dd integrations
		return nil, errors.New("pushing argo application with multiple destinations is currently unsupported")
	}
	var (
		datadogIntegration *integration.DataDogIntegrationData
		err                error
	)
	for _, destination := range connection.Destinations {
		switch destination.DestinationType {
		case "datadog":
			datadogIntegration, err = oc.datadogIntegration.GetIntegrationByName(ctx, destination.IntegrationName)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown destination type: %s", destination.DestinationType)
		}
	}

	templateData := ArgoConnectionTemplateData{
		AppName:                name,
		CurrentNamespace:       oc.configuration.CurrentNamespace,
		ServiceAccount:         oc.configuration.ServiceAccountName,
		Namespace:              connection.MdaiNamespace,
		ConnectionData:         connection,
		DatadogIntegrationData: datadogIntegration,
		// Tells template to manually inject Argo tracking annotations. We only want these for direct sync force push
		IsArgoSideload: connection.Deployment.Type == ArgoSideloadDeploymentType,
	}
	return &templateData, nil
}

type ManifestGenerator interface {
	CreateExportableArgoManifests(input CompressionInput, connection OctantConnectionData) (map[string][]byte, error)
	CreateExportableTemplateData(
		name string,
		connection OctantConnectionData,
	) (*ArgoConnectionTemplateData, error)
	RenderMdaiAppManifest(mdaiVersion, namespace string) ([]byte, error)
	RenderCollectorDeploymentManifests(
		templateData *ArgoConnectionTemplateData,
		outputFormat ManifestOutputFormat,
	) (map[string][]byte, error)
	RenderValidatorManifestForConnection(
		templateData *ArgoValidatorTemplateData,
		outputFormat ManifestOutputFormat,
	) ([]byte, error)
	RenderArgoAppManifest(
		templateData *ArgoConnectionTemplateData,
		outputFormat ManifestOutputFormat,
	) ([]byte, error)
}

// ConnectionManifestGenerator implements ManifestCompressor.
type ConnectionManifestGenerator struct {
	configuration *config.Configuration
}

// Ensure ConnectionManifestGenerator implements ManifestCompressor.
var _ ManifestGenerator = &ConnectionManifestGenerator{}

// NewConnectionManifestGenerator returns a new instance of ConnectionManifestGenerator.
func NewConnectionManifestGenerator(con *config.Configuration) *ConnectionManifestGenerator {
	return &ConnectionManifestGenerator{
		configuration: con,
	}
}

func (cmg *ConnectionManifestGenerator) CreateExportableArgoManifests(
	input CompressionInput,
	connection OctantConnectionData,
) (map[string][]byte, error) {
	format := cmg.toConnectionFormat(input.Format)
	templateData, err := cmg.CreateExportableTemplateData(input.Connection, connection)
	if err != nil {
		return nil, err
	}

	manifests, err := cmg.RenderCollectorDeploymentManifests(templateData, format)
	if err != nil {
		return nil, err
	}
	validatorTemplateData := ArgoValidatorTemplateData{
		ConnectionName: input.Connection,
		Namespace:      input.Namespace,
		ValidatorRunID: getRunID(),
	}
	validatorManifest, err := cmg.RenderValidatorManifestForConnection(&validatorTemplateData, format)
	if err != nil {
		return nil, err
	}
	manifests[cmg.getFilename("validator", format)] = validatorManifest
	appManifest, err := cmg.RenderArgoAppManifest(templateData, format)
	if err != nil {
		return nil, err
	}
	manifests[fmt.Sprintf("argo-app.%s", format)] = appManifest

	mdaiManifest, err := cmg.RenderMdaiAppManifest(input.MdaiVersion, input.Namespace)
	if err != nil {
		return nil, err
	}
	manifests[fmt.Sprintf("mdai-app.%s", format)] = mdaiManifest
	return manifests, nil
}

// CreateExportableTemplateData TODO: Combine these template data methods instead of copypasta
// CreateExportableTemplateData is like the other function but doesn't inject secrets.
func (cmg *ConnectionManifestGenerator) CreateExportableTemplateData(
	name string,
	connection OctantConnectionData,
) (*ArgoConnectionTemplateData, error) {
	if len(connection.Destinations) != 1 {
		// TODO: Implement multiple destination handling and handling of non-dd integrations
		return nil, errors.New("generating argo application with multiple destinations is currently unsupported")
	}
	datadogIntegration := integration.DataDogIntegrationData{ // nolint:gosec // no, these are not secrets lol
		APIKey: "<YOUR_API_KEY>",
		DDUrl:  "<YOUR_DD_URL>",
	}

	templateData := ArgoConnectionTemplateData{
		AppName:                name,
		CurrentNamespace:       cmg.configuration.CurrentNamespace,
		ServiceAccount:         cmg.configuration.ServiceAccountName,
		Namespace:              connection.MdaiNamespace,
		ConnectionData:         connection,
		DatadogIntegrationData: &datadogIntegration,
		// Tells template to manually inject Argo tracking annotations. We only want these for direct sync force push
		IsArgoSideload: connection.Deployment.Type == ArgoSideloadDeploymentType,
	}
	return &templateData, nil
}

// RenderMdaiAppManifest renders the mdai argo application manifest with the provided template inputs.
func (*ConnectionManifestGenerator) RenderMdaiAppManifest(mdaiVersion, namespace string) ([]byte, error) {
	appManifestTemplate, err := template.New("mdai-app").Parse(mdaiAppTemplate)
	if err != nil {
		return []byte{}, fmt.Errorf("error parsing mdai app template: %w", err)
	}

	var renderedYaml bytes.Buffer
	templateData := struct {
		MdaiVersion string
		Namespace   string
	}{
		MdaiVersion: mdaiVersion,
		Namespace:   namespace,
	}
	if templateErr := appManifestTemplate.Execute(&renderedYaml, templateData); templateErr != nil {
		return []byte{}, templateErr
	}

	return renderedYaml.Bytes(), nil
}

// RenderArgoAppManifest renders an argo app manifest which establishes a repo for syncing octant manifests with.
func (*ConnectionManifestGenerator) RenderArgoAppManifest(
	templateData *ArgoConnectionTemplateData,
	outputFormat ManifestOutputFormat,
) ([]byte, error) {
	if outputFormat == "" {
		return []byte{}, errors.New("no output format specified")
	}
	appManifestTemplate, err := template.New("argo-app").Parse(argoAppTemplate)
	if err != nil {
		return []byte{}, err
	}
	var renderedYaml bytes.Buffer
	if templateErr := appManifestTemplate.Execute(&renderedYaml, templateData); templateErr != nil {
		return []byte{}, templateErr
	}

	switch outputFormat {
	case YAMLOutputFormat:
		return renderedYaml.Bytes(), nil
	case JSONOutputFormat:
		renderedJSON, err := yaml.YAMLToJSON(renderedYaml.Bytes())
		if err != nil {
			return []byte{}, err
		}

		return renderedJSON, nil
	}

	return renderedYaml.Bytes(), nil
}

func (cmg *ConnectionManifestGenerator) RenderCollectorDeploymentManifests(
	templateData *ArgoConnectionTemplateData,
	outputFormat ManifestOutputFormat,
) (map[string][]byte, error) {
	if outputFormat == "" {
		return nil, errors.New("no output format specified")
	}

	templates := map[string]string{
		"lb-collector":    lbCollectorTemplate,
		"log-collector":   logCollectorTemplate,
		"trace-collector": traceCollectorTemplate,
		"hub":             hubTemplate,
		"observer":        observerTemplate,
		"secret":          secretTemplate,
		"additional":      additionalTemplate,
	}

	manifests := make(map[string][]byte)
	for templateName, templateString := range templates {
		appManifestTemplate, err := template.New(templateName).Parse(templateString)
		if err != nil {
			return manifests, err
		}

		var renderedYaml bytes.Buffer
		if templateErr := appManifestTemplate.Execute(&renderedYaml, templateData); templateErr != nil {
			return manifests, templateErr
		}

		filename := cmg.getFilename(templateName, outputFormat)
		switch outputFormat {
		case YAMLOutputFormat:
			manifests[filename] = renderedYaml.Bytes()
		case JSONOutputFormat:
			renderedJSON, err := yaml.YAMLToJSON(renderedYaml.Bytes())
			if err != nil {
				return manifests, err
			}

			manifests[filename] = renderedJSON
		default:
			return manifests, errors.New("unknown output format")
		}
	}

	return manifests, nil
}

func (*ConnectionManifestGenerator) RenderValidatorManifestForConnection(
	templateData *ArgoValidatorTemplateData,
	outputFormat ManifestOutputFormat,
) ([]byte, error) {
	if outputFormat == "" {
		return nil, errors.New("no output format specified")
	}

	var manifest []byte
	appManifestTemplate, err := template.New("validator").Parse(validatorTemplate)
	if err != nil {
		return nil, err
	}

	var renderedYaml bytes.Buffer
	if templateErr := appManifestTemplate.Execute(&renderedYaml, templateData); templateErr != nil {
		return nil, templateErr
	}

	switch outputFormat {
	case YAMLOutputFormat:
		manifest = renderedYaml.Bytes()
	case JSONOutputFormat:
		renderedJSON, err := yaml.YAMLToJSON(renderedYaml.Bytes())
		if err != nil {
			return nil, err
		}

		manifest = renderedJSON
	default:
		return nil, errors.New("unknown output format")
	}

	return manifest, nil
}

// toConnectionFormat convertsManifestOutFormat enum to ManifestOutputFormat.
func (*ConnectionManifestGenerator) toConnectionFormat(format octantv1alpha.ManifestOutFormat) ManifestOutputFormat {
	result := YAMLOutputFormat
	if format == octantv1alpha.ManifestOutFormat_MANIFEST_OUT_FORMAT_JSON {
		result = JSONOutputFormat
	}

	return result
}

func (*ConnectionManifestGenerator) getFilename(templateName string, outputFormat ManifestOutputFormat) string {
	return fmt.Sprintf("%s.%s", templateName, outputFormat)
}
