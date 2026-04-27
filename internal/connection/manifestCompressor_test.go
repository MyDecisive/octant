package connection

import (
	"testing"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionManifestCompressor_CreateCompressed(t *testing.T) {
	t.Parallel()

	target := NewConnectionManifestCompressor()
	actual, err := target.CreateCompressed(t.Context(), CompressionInput{
		Namespace:  faker.Word(),
		Connection: faker.Word(),
		Telemetries: []octantv1alpha.MLTType{
			octantv1alpha.MLTType_MLT_TYPE_METRIC,
		},
		DeploymentType: octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_SIDELOAD,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, actual)
}

func TestConnectionManifestCompressor_ToConnectionData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		des      string
		in       octantv1alpha.DeploymentType
		expected DeploymentType
	}{
		{"json", octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_MANIFEST, ArgoManifestsDeploymentType},
		{"yaml", octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_SIDELOAD, ArgoSideloadDeploymentType},
	}
	for _, tt := range tests {
		t.Run(tt.des, func(t *testing.T) {
			t.Parallel()
			tel := []octantv1alpha.MLTType{
				octantv1alpha.MLTType_MLT_TYPE_METRIC,
				octantv1alpha.MLTType_MLT_TYPE_LOG,
				octantv1alpha.MLTType_MLT_TYPE_TRACE,
			}
			target := NewConnectionManifestCompressor()
			actual := target.toConnectionData(tel, tt.in)

			assert.Len(t, actual.TelemetryTypes, 3)
			assert.Contains(t, actual.TelemetryTypes, telemetry.Metrics)
			assert.Contains(t, actual.TelemetryTypes, telemetry.Logs)
			assert.Contains(t, actual.TelemetryTypes, telemetry.Traces)
			assert.Len(t, actual.Destinations, 1)
			assert.Equal(t, tt.expected, actual.Deployment.Type)
		})
	}
}

func TestConnectionManifestCompressor_ToConnectionFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		des      string
		in       octantv1alpha.ManifestOutFormat
		expected ManifestOutputFormat
	}{
		{"json", octantv1alpha.ManifestOutFormat_MANIFEST_OUT_FORMAT_JSON, JSONOutputFormat},
		{"yaml", octantv1alpha.ManifestOutFormat_MANIFEST_OUT_FORMAT_YAML, YAMLOutputFormat},
	}
	for _, tt := range tests {
		t.Run(tt.des, func(t *testing.T) {
			t.Parallel()
			target := NewConnectionManifestCompressor()
			actual := target.toConnectionFormat(tt.in)

			assert.Equal(t, tt.expected, actual)
		})
	}
}
