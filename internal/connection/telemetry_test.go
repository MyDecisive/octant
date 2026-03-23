package connection

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	type testTelemetry struct {
		TheTelemetry Telemetry `json:"theTelemetry"`
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
