package manifest

import (
	"testing"

	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgoCDManager_GetAppName(t *testing.T) {
	t.Parallel()

	for _, tt := range manifestdata.AppValues() {
		t.Run(tt.String(), func(t *testing.T) {
			t.Parallel()

			target := NewArgoCDManager(nil, nil, nil, nil)
			expected := target.getAppName(tt, "{{ .Name }}")

			actual, err := templates.ReadFile("template/" + tt.String() + ".yaml.tmpl")
			require.NoError(t, err)
			assert.Contains(t, string(actual), expected)
		})
	}
}
