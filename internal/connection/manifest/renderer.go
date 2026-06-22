package manifest

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	tmpl "text/template"

	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// TemplateRenderer renders any generic template.
type TemplateRenderer interface {
	// Render renders the provided template using the provided data.
	// Note: format will define the format of the rendered manifest.
	Render(name string, template []byte, format manifestdata.OutputFormat, data any) ([][]byte, error)
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
	format manifestdata.OutputFormat,
	data any,
) ([][]byte, error) {
	gen, err := tmpl.New(name).Parse(string(template))
	if err != nil {
		return nil, fmt.Errorf("%w:%w", ErrParseTemplate, err)
	}

	var render bytes.Buffer
	if err = gen.Execute(&render, data); err != nil {
		return nil, fmt.Errorf("%w:%w", ErrRenderTemplate, err)
	}

	yml := render.Bytes()
	out := [][]byte{}
	if format == manifestdata.JSON {
		reader := kyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(yml)))
		for {
			raw, err := reader.Read()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return out, nil
				}
				return nil, fmt.Errorf("%w:%w", ErrConvertJSON, err)
			}
			jsn, err := yaml.YAMLToJSON(raw)
			if err != nil {
				return nil, fmt.Errorf("%w:%w", ErrConvertJSON, err)
			}
			out = append(out, jsn)
		}
	}
	out = append(out, yml)
	return out, nil
}
