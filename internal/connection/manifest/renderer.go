package manifest

import (
	"bytes"
	"fmt"
	tmpl "text/template"

	manfiestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"sigs.k8s.io/yaml"
)

// TemplateRenderer renders any generic template.
type TemplateRenderer interface {
	// Render renders the provided template using the provided data.
	// Note: format will define the format of the rendered manifest.
	Render(name string, template []byte, format manfiestdata.OutputFormat, data any) ([]byte, error)
}

// TextTemplateRenderer implements Renderer using "text/template" package.
type TextTemplateRenderer struct{}

// Ensure TextTemplateRenderer implements TemplateRenderer.
var _ TemplateRenderer = (*TextTemplateRenderer)(nil)

// NewTextTemplateRenderer returns a new instance of TextTemplateRenderer.
func NewTextTemplateRenderer() *TextTemplateRenderer {
	return &TextTemplateRenderer{}
}

// Render renders the provided template using the provided data.
// Note: format will define the format of the rendered manifest.
func (*TextTemplateRenderer) Render(
	name string,
	template []byte,
	format manfiestdata.OutputFormat,
	data any,
) ([]byte, error) {
	gen, err := tmpl.New(name).Parse(string(template))
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrParseTemplate, err)
	}

	var render bytes.Buffer
	if err = gen.Execute(&render, data); err != nil {
		return nil, fmt.Errorf("%w:%w", ErrRenderTemplate, err)
	}

	out := render.Bytes()
	if format == manfiestdata.JSON {
		out, err = yaml.YAMLToJSON(out)
		if err != nil {
			return nil, fmt.Errorf("%w:%w", ErrConvertJSON, err)
		}
	}
	return out, nil
}
