package connection

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"text/template"

	"github.com/mydecisive/octant/internal/integration"
	"sigs.k8s.io/yaml"
)

//go:embed manifests/cert-manager.yaml
var CertManagerAppManifest []byte

//go:embed templates/argo-app.yaml.tmpl
var argoAppTemplate string

//go:embed templates/mdai-app.yaml.tmpl
var mdaiAppTemplate string

//go:embed templates/collector.yaml.tmpl
var primaryCollectorTemplate string

//go:embed templates/hub.yaml.tmpl
var hubTemplate string

//go:embed templates/validator.yaml.tmpl
var validatorTemplate string

//go:embed templates/secret.yaml.tmpl
var secretTemplate string

type ArgoTemplateData struct {
	AppName                string
	Namespace              string
	ConnectionData         OctantConnectionData
	DatadogIntegrationData *integration.DataDogIntegrationData
	ValidatorEnabled       bool
	IsArgoSideload         bool
}

type ManifestOutputFormat string

const (
	YAMLOutputFormat ManifestOutputFormat = "yaml"
	JSONOutputFormat ManifestOutputFormat = "json"
)

func (oc *OctantConnection) createTemplateData(
	ctx context.Context,
	namespace string,
	name string,
	connection OctantConnectionData,
) (*ArgoTemplateData, error) {
	if len(connection.Destinations) != 1 {
		// TODO: Implement multiple destination handling and handling of non-dd integrations
		return nil, errors.New("pushing argo application with multiple destinations is currently unsupported")
	}
	var datadogIntegration *integration.DataDogIntegrationData
	for _, destination := range connection.Destinations {
		switch destination.DestinationType {
		case "datadog":
			foundDDIntegration, getDDIntErr := oc.datadogClient.GetIntegrationByName(ctx, namespace, destination.IntegrationName)
			if getDDIntErr != nil {
				return nil, getDDIntErr
			}
			datadogIntegration = foundDDIntegration
		default:
			return nil, fmt.Errorf("unknown destination type: %s", destination.DestinationType)
		}
	}

	templateData := ArgoTemplateData{
		AppName:                name,
		Namespace:              namespace,
		ConnectionData:         connection,
		DatadogIntegrationData: datadogIntegration,
		ValidatorEnabled:       true,
		// Tells template to manually inject Argo tracking annotations. We only want these for direct sync force push
		IsArgoSideload: connection.Deployment.Type == ArgoSideloadDeploymentType,
	}
	return &templateData, nil
}

func CreateExportableArgoManifests(
	namespace string,
	name string,
	connection OctantConnectionData,
	format ManifestOutputFormat,
) (map[string][]byte, error) {
	templateData, err := CreateExportableTemplateData(namespace, name, connection)
	if err != nil {
		return nil, err
	}
	manifests, err := renderCollectorDeploymentManifests(templateData, format)
	if err != nil {
		return nil, err
	}
	appManifest, err := renderArgoAppManifest(templateData, format)
	if err != nil {
		return nil, err
	}
	manifests[fmt.Sprintf("argo-app.%s", format)] = appManifest
	return manifests, nil
}

// CreateExportableTemplateData TODO: Combine these template data methods instead of copypasta
// CreateExportableTemplateData is like the other function but doesn't inject secrets.
func CreateExportableTemplateData(
	namespace string,
	name string,
	connection OctantConnectionData,
) (*ArgoTemplateData, error) {
	if len(connection.Destinations) != 1 {
		// TODO: Implement multiple destination handling and handling of non-dd integrations
		return nil, errors.New("pushing argo application with multiple destinations is currently unsupported")
	}
	datadogIntegration := integration.DataDogIntegrationData{ // nolint:gosec // no, these are not secrets lol
		APIKey: "<YOUR_API_KEY>",
		DDUrl:  "<YOUR_DD_URL>",
	}

	templateData := ArgoTemplateData{
		AppName:                name,
		Namespace:              namespace,
		ConnectionData:         connection,
		DatadogIntegrationData: &datadogIntegration,
		ValidatorEnabled:       true,
		// Tells template to manually inject Argo tracking annotations. We only want these for direct sync force push
		IsArgoSideload: connection.Deployment.Type == ArgoSideloadDeploymentType,
	}
	return &templateData, nil
}

// RenderMdaiAppManifest renders the mdai argo application manifest with the provided template inputs.
func RenderMdaiAppManifest(mdaiVersion, namespace string) ([]byte, error) {
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

func renderArgoAppManifest(templateData *ArgoTemplateData, outputFormat ManifestOutputFormat) ([]byte, error) {
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

func renderCollectorDeploymentManifests(
	templateData *ArgoTemplateData,
	outputFormat ManifestOutputFormat,
) (map[string][]byte, error) {
	if outputFormat == "" {
		return nil, errors.New("no output format specified")
	}

	manifests := make(map[string][]byte)
	for templateName, templateString := range map[string]string{
		"collector": primaryCollectorTemplate,
		"hub":       hubTemplate,
		"validator": validatorTemplate,
		"secret":    secretTemplate,
	} {
		appManifestTemplate, err := template.New(templateName).Parse(templateString)
		if err != nil {
			return manifests, err
		}

		var renderedYaml bytes.Buffer
		if templateErr := appManifestTemplate.Execute(&renderedYaml, templateData); templateErr != nil {
			return manifests, templateErr
		}

		filename := fmt.Sprintf("%s.%s", templateName, outputFormat)
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
