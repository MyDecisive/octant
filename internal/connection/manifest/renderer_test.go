package manifest

import (
	"fmt"
	"testing"

	"github.com/go-faker/faker/v4"
	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextTemplateRenderer_Render(t *testing.T) {
	t.Parallel()

	singleFormatter := "metadata:\n name: %s"
	multiFormatter := "metadata:\n name: %[1]s\n---\nmetadata:\n name: %[1]s"
	jsonFormatter := "{\"metadata\":{\"name\":%q}}"
	replacer := "{{ .Name }}"

	name := faker.Word()
	data := struct {
		Name string
	}{
		Name: name,
	}

	t.Run("success yaml single", func(t *testing.T) {
		t.Parallel()

		expected := fmt.Appendf([]byte{}, singleFormatter, name)

		target := NewTextTemplateRenderer()

		actual, err := target.Render(name, fmt.Appendf([]byte{}, singleFormatter, replacer), manifestdata.YAML, data)
		require.NoError(t, err)

		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success yaml multi", func(t *testing.T) {
		t.Parallel()

		expected := fmt.Appendf([]byte{}, multiFormatter, name)

		target := NewTextTemplateRenderer()

		actual, err := target.Render(name, fmt.Appendf([]byte{}, multiFormatter, replacer), manifestdata.YAML, data)
		require.NoError(t, err)

		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success json single", func(t *testing.T) {
		t.Parallel()

		expected := fmt.Appendf([]byte{}, jsonFormatter, name)

		target := NewTextTemplateRenderer()

		actual, err := target.Render(name, fmt.Appendf([]byte{}, singleFormatter, replacer), manifestdata.JSON, data)
		require.NoError(t, err)

		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success json multi", func(t *testing.T) {
		t.Parallel()

		expected := fmt.Appendf([]byte{}, jsonFormatter, name)

		target := NewTextTemplateRenderer()

		actual, err := target.Render(name, fmt.Appendf([]byte{}, multiFormatter, replacer), manifestdata.JSON, data)
		require.NoError(t, err)

		assert.Len(t, actual, 2)
		assert.Equal(t, expected, actual[0])
		assert.Equal(t, expected, actual[1])
	})

	t.Run("err render template", func(t *testing.T) {
		t.Parallel()

		target := NewTextTemplateRenderer()

		actual, err := target.Render(name, []byte("{{.TEST}}"), manifestdata.JSON, data)
		assert.Nil(t, actual)
		assert.ErrorIs(t, err, ErrRenderTemplate)
	})

	t.Run("err convert json", func(t *testing.T) {
		t.Parallel()

		target := NewTextTemplateRenderer()

		actual, err := target.Render(name, []byte("}"), manifestdata.JSON, data)
		assert.Nil(t, actual)
		assert.ErrorIs(t, err, ErrConvertJSON)
	})
}
