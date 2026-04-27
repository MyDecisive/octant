package connection

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/octant/internal/telemetry"
)

type CompressionInput struct {
	Namespace      string
	Connection     string
	Telemetries    []octantv1alpha.MLTType
	Format         octantv1alpha.ManifestOutFormat
	DeploymentType octantv1alpha.DeploymentType
}

// ManifestCompressor provide functionality to generate a compressed connection manifest.
type ManifestCompressor interface {
	// CreateCompressed creates manifest files abse on the given inputs and then compress the files into a zip.
	CreateCompressed(ctx context.Context, input CompressionInput) (*bytes.Buffer, error)
}

// ConnectionManifestCompressor implements ManifestCompressor.
type ConnectionManifestCompressor struct{}

// Ensure ConnectionManifestCompressor implements ManifestCompressor.
var _ ManifestCompressor = &ConnectionManifestCompressor{}

// NewConnectionManifestCompressor returns a new instance of ConnectionManifestCompressor.
func NewConnectionManifestCompressor() *ConnectionManifestCompressor {
	return &ConnectionManifestCompressor{}
}

// CreateCompressed creates manifest files abse on the given inputs and then compress the files into a zip.
func (cmc *ConnectionManifestCompressor) CreateCompressed(
	ctx context.Context,
	input CompressionInput,
) (*bytes.Buffer, error) {
	manifestsMap, err := CreateExportableArgoManifests(
		input.Namespace,
		input.Connection,
		cmc.toConnectionData(input.Telemetries, input.DeploymentType),
		cmc.toConnectionFormat(input.Format),
	)
	if err != nil {
		return nil, fmt.Errorf("render manifest:%w", err)
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	defer zipWriter.Close() //nolint:errcheck
	for filename, content := range manifestsMap {
		select {
		case <-ctx.Done():
			return nil, errors.New("context cancelled")
		default:
		}

		fWriter, err := zipWriter.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("generate zip for %s:%w", filename, err)
		}

		if _, err := fWriter.Write(content); err != nil {
			return nil, fmt.Errorf("write zip for %s:%w", filename, err)
		}
	}
	return buf, nil
}

// toConnectionData converts the telemetry and deployment type to connection data.
func (*ConnectionManifestCompressor) toConnectionData(
	telemetries []octantv1alpha.MLTType,
	deployment octantv1alpha.DeploymentType,
) OctantConnectionData {
	telemetryTypes := make([]telemetry.MLT, len(telemetries))
	for i, t := range telemetries {
		switch t {
		case octantv1alpha.MLTType_MLT_TYPE_METRIC:
			telemetryTypes[i] = telemetry.Metrics
		case octantv1alpha.MLTType_MLT_TYPE_LOG:
			telemetryTypes[i] = telemetry.Logs
		case octantv1alpha.MLTType_MLT_TYPE_TRACE:
			telemetryTypes[i] = telemetry.Traces
		}
	}

	deploymentType := ArgoSideloadDeploymentType
	if deployment == octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_MANIFEST {
		deploymentType = ArgoManifestsDeploymentType
	}

	return OctantConnectionData{
		TelemetryTypes: telemetryTypes,
		Deployment: &Deployment{
			Type: deploymentType,
		},
		Destinations: make([]OctantConnectionDestination, 1),
	}
}

// toConnectionFormat convertsManifestOutFormat enum to ManifestOutputFormat.
func (*ConnectionManifestCompressor) toConnectionFormat(format octantv1alpha.ManifestOutFormat) ManifestOutputFormat {
	result := YAMLOutputFormat
	if format == octantv1alpha.ManifestOutFormat_MANIFEST_OUT_FORMAT_JSON {
		result = JSONOutputFormat
	}

	return result
}
