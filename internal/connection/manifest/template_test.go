package manifest

import (
	"io/fs"
	"strings"
	"testing"

	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test is not testing the EmbeddedTemplateProvider per se
// But it is here to ensures that adding all template enums together will
// cover all template files in the template folder.
func TestEmbeddedTemplateProvider_CoverAllTemplates(t *testing.T) {
	t.Parallel()

	expected := manifestdata.AppStrings()
	expected = append(expected, manifestdata.ConnectionStrings()...)
	expected = append(expected, manifestdata.ValidatorStrings()...)

	dir, err := templates.ReadDir("template")
	require.NoError(t, err)
	actual := lo.Map(dir, func(item fs.DirEntry, _ int) string {
		return strings.TrimSuffix(item.Name(), ".yaml.tmpl")
	})

	assert.Len(t, actual, len(expected))
	assert.ElementsMatch(t, expected, actual)
}

func TestEmbeddedTemplateProvider_GetApp(t *testing.T) {
	t.Parallel()

	target := NewEmbeddedTemplateProvider()

	for _, tt := range manifestdata.AppValues() {
		t.Run(tt.String(), func(t *testing.T) {
			t.Parallel()
			expected, err := templates.ReadFile("template/" + tt.String() + ".yaml.tmpl")
			require.NoError(t, err)

			actual, err := target.GetApp(tt)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestEmbeddedTemplateProvider_GetAllConnections(t *testing.T) {
	t.Parallel()

	target := NewEmbeddedTemplateProvider()

	expected := make(map[manifestdata.Connection][]byte)
	for _, conn := range manifestdata.ConnectionValues() {
		val, err := templates.ReadFile("template/" + conn.String() + ".yaml.tmpl")
		require.NoError(t, err)
		expected[conn] = val
	}

	actual, err := target.GetAllConnections()
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestEmbeddedTemplateProvider_GetAllValidators(t *testing.T) {
	t.Parallel()

	target := NewEmbeddedTemplateProvider()

	expected := make(map[manifestdata.Validator][]byte)
	for _, va := range manifestdata.ValidatorValues() {
		val, err := templates.ReadFile("template/" + va.String() + ".yaml.tmpl")
		require.NoError(t, err)
		expected[va] = val
	}

	actual, err := target.GetAllValidators()
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}
