package connection

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"time"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/mydecisive/octant/internal/telemetry"
)

type CompressionInput struct {
	Namespace      string
	Connection     string
	MdaiVersion    string
	Telemetries    []octantv1alpha.MLTType
	Format         octantv1alpha.ManifestOutFormat
	DeploymentType octantv1alpha.DeploymentType
}

// ManifestCompressor provide functionality to generate a compressed connection manifest.
type ManifestCompressor interface {
	// CreateCompressed creates manifest files based on the given inputs and then compress the files into a zip.
	CreateCompressed(ctx context.Context, input CompressionInput) (*bytes.Buffer, error)
}

// ConnectionManifestCompressor implements ManifestCompressor.
type ConnectionManifestCompressor struct {
	generator ManifestGenerator
}

// Ensure ConnectionManifestCompressor implements ManifestCompressor.
var _ ManifestCompressor = &ConnectionManifestCompressor{}

// NewConnectionManifestCompressor returns a new instance of ConnectionManifestCompressor.
func NewConnectionManifestCompressor(generator ManifestGenerator) *ConnectionManifestCompressor {
	return &ConnectionManifestCompressor{
		generator: generator,
	}
}

// CreateCompressed creates manifest files based on the given inputs and then compresses the files into a zip.
func (cmc *ConnectionManifestCompressor) CreateCompressed(
	ctx context.Context,
	input CompressionInput,
) (*bytes.Buffer, error) {
	data := cmc.toConnectionData(input.Telemetries, input.DeploymentType)
	data.MdaiNamespace = input.Namespace
	manifestsMap, err := cmc.generator.CreateExportableArgoManifests(
		input,
		data,
	)
	if err != nil {
		return nil, fmt.Errorf("render manifest:%w", err)
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for filename, content := range manifestsMap {
		select {
		case <-ctx.Done():
			return nil, errors.New("context cancelled")
		default:
		}

		// Manually compress the content in memory first
		var compressedBuf bytes.Buffer
		flateWriter, err := flate.NewWriter(&compressedBuf, flate.DefaultCompression)
		if err != nil {
			return nil, fmt.Errorf("create flate writer for %s:%w", filename, err)
		}
		if _, err := flateWriter.Write(content); err != nil {
			return nil, fmt.Errorf("flate write for %s:%w", filename, err)
		}
		if err := flateWriter.Close(); err != nil {
			return nil, fmt.Errorf("flate close for %s:%w", filename, err)
		}

		compressedContent := compressedBuf.Bytes()

		// Now we know the exact compressed size. Build the header.
		header := &zip.FileHeader{
			Name:               filename,
			Method:             zip.Deflate,
			UncompressedSize64: uint64(len(content)),
			CompressedSize64:   uint64(len(compressedContent)),
			CRC32:              crc32.ChecksumIEEE(content),
			CreatorVersion:     20,    // v2.0
			ReaderVersion:      20,    // v2.0
			Flags:              0x800, // Explicitly declare UTF-8 filenames (Bit 11)
		}

		// SetModTime converts time.Now() into the legacy MS-DOS uint16 fields
		// that CreateRaw actually writes to the byte stream.
		// Uses deprecated `SetModTime` due to https://github.com/golang/go/issues/76741
		header.SetModTime(time.Now())

		// SetMode establishes standard read/write file permissions (ExternalAttrs)
		// which Windows Explorer relies on to know it's a standard file.
		header.SetMode(0644)

		// Use CreateRaw to inject the pre-compressed bytes directly
		fWriter, err := zipWriter.CreateRaw(header)
		if err != nil {
			return nil, fmt.Errorf("generate zip header for %s:%w", filename, err)
		}

		if _, err := fWriter.Write(compressedContent); err != nil {
			return nil, fmt.Errorf("write compressed zip data for %s:%w", filename, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
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
