package manifest

import (
	"embed"
	"fmt"

	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
)

//go:embed template/*.yaml.tmpl
var templates embed.FS

const (
	validatorAppNameFormatter = "%s-validation"
	templateFileFormatter     = "template/%s.yaml.tmpl"
)

type TemplateProvider interface {
	// GetApp returns content of the ArgoCD app template file correspond to the provided app template type.
	GetApp(app manifestdata.App) ([]byte, error)
	// GetConnection returns content of the template file correspond to the provided connection template type.
	GetConnection(conn manifestdata.Connection) ([]byte, error)
	// GetAllConnections returns content of the template files for all connection template types.
	GetAllConnections() (map[manifestdata.Connection][]byte, error)
	// GetValidator returns content of the template file correspond to the provided validator template type.
	GetValidator(validator manifestdata.Validator) ([]byte, error)
	// GetAllValidators returns content of the template files for all validator template types.
	GetAllValidators() (map[manifestdata.Validator][]byte, error)
}

// EmbeddedTemplateProvider implements TemplateProvider using embedded file system.
type EmbeddedTemplateProvider struct{}

// Ensure EmbeddedTemplateProvider implements TemplateProvider.
var _ TemplateProvider = (*EmbeddedTemplateProvider)(nil)

// NewEmbeddedTemplateProvider returns a new instance of EmbeddedTemplateProvider.
func NewEmbeddedTemplateProvider() *EmbeddedTemplateProvider {
	return &EmbeddedTemplateProvider{}
}

// GetApp returns content of the ArgoCD app template file correspond to the provided app template type.
func (mtp *EmbeddedTemplateProvider) GetApp(app manifestdata.App) ([]byte, error) {
	return mtp.format(app.String())
}

// GetConnection returns content of the template file correspond to the provided connection template type.
func (mtp *EmbeddedTemplateProvider) GetConnection(conn manifestdata.Connection) ([]byte, error) {
	return mtp.format(conn.String())
}

// GetAllConnections returns content of the template files for all connection template types.
func (mtp *EmbeddedTemplateProvider) GetAllConnections() (map[manifestdata.Connection][]byte, error) {
	result := make(map[manifestdata.Connection][]byte)
	for _, conn := range manifestdata.ConnectionValues() {
		content, err := mtp.GetConnection(conn)
		if err != nil {
			return nil, fmt.Errorf("%s:%w", conn.String(), err)
		}
		result[conn] = content
	}
	return result, nil
}

// GetValidator returns content of the template file correspond to the provided validator template type.
func (mtp *EmbeddedTemplateProvider) GetValidator(validator manifestdata.Validator) ([]byte, error) {
	return mtp.format(validator.String())
}

// GetAllValidators returns content of the template files for all validator template types.
func (mtp *EmbeddedTemplateProvider) GetAllValidators() (map[manifestdata.Validator][]byte, error) {
	result := make(map[manifestdata.Validator][]byte)
	for _, val := range manifestdata.ValidatorValues() {
		content, err := mtp.GetValidator(val)
		if err != nil {
			return nil, fmt.Errorf("%s:%w", val.String(), err)
		}
		result[val] = content
	}
	return result, nil
}

func (*EmbeddedTemplateProvider) format(prefix string) ([]byte, error) {
	name := fmt.Sprintf(templateFileFormatter, prefix)
	return templates.ReadFile(name)
}
