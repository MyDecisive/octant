package telemetry

import (
	"encoding/json"
	"testing"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	type testTelemetry struct {
		TheTelemetry MLT `json:"theTelemetry"`
	}

	t.Run("invalid telemetry", func(t *testing.T) {
		t.Parallel()

		theJSON := `{"theTelemetry": "invalid"}`

		var test testTelemetry
		require.ErrorContains(t, json.Unmarshal([]byte(theJSON), &test), "invalid telemetry type: invalid")
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		theJSON := `{"theTelemetry": "traces"}`

		var test testTelemetry
		require.NoError(t, json.Unmarshal([]byte(theJSON), &test))
		assert.Equal(t, Traces, test.TheTelemetry)
	})
}

func TestToMLTs(t *testing.T) {
	t.Parallel()

	input := []octantv1alpha.MLTType{
		octantv1alpha.MLTType_MLT_TYPE_LOG,
		octantv1alpha.MLTType_MLT_TYPE_METRIC,
		octantv1alpha.MLTType_MLT_TYPE_TRACE,
	}

	actual := ToMLTs(input)
	assert.Len(t, actual, 3)
	assert.Contains(t, actual, Logs)
	assert.Contains(t, actual, Metrics)
	assert.Contains(t, actual, Traces)
}
