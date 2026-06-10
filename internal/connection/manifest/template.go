package manifest

import (
	"embed"
	"fmt"
)

// App defines the possible ArgoCD apps octant can generate.
//
//go:generate enumer -type=App -addprefix=app- -transform=lower -text
type App int // nolint: recvcheck // the methods are generated
const (
	MDAI App = iota
	CERT
	CONNECTION
	VALIDATOR
)

// Connection defines the possible connection app specific templates octant can generate.
//
//go:generate enumer -type=Connection -addprefix=connection- -transform=lower -text
type Connection int // nolint: recvcheck // the methods are generated
const (
	HUB Connection = iota
	OBSERVER
	ROLE
	SECRET
	COLLECTORLB
	COLLECTORLOG
	COLLECTORTRACE
)

// Validator defines the possible validator app specific templates octant can generate.
//
//go:generate enumer -type=Validator -addprefix=validator- -transform=lower -text
type Validator int // nolint: recvcheck // the methods are generated
const (
	TELEMETRY Validator = iota
)

//go:embed template/*.yaml.tmpl
var templates embed.FS

const templateFileFormatter = "template/%s.yaml.tmpl"

type TemplateProvider interface {
	// GetApp returns content of the ArgoCD app template file correspond to the provided app template type.
	GetApp(app App) ([]byte, error)
	// GetConnection returns content of the template file correspond to the provided connection template type.
	GetConnection(conn Connection) ([]byte, error)
	// GetAllConnections returns content of the template files for all connection template types.
	GetAllConnections() (map[Connection][]byte, error)
	// GetValidator returns content of the template file correspond to the provided validator template type.
	GetValidator(validator Validator) ([]byte, error)
	// GetAllValidators returns content of the template files for all validator template types.
	GetAllValidators() (map[Validator][]byte, error)
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
func (mtp *EmbeddedTemplateProvider) GetApp(app App) ([]byte, error) {
	return mtp.format(app.String())
}

// GetConnection returns content of the template file correspond to the provided connection template type.
func (mtp *EmbeddedTemplateProvider) GetConnection(conn Connection) ([]byte, error) {
	return mtp.format(conn.String())
}

// GetAllConnections returns content of the template files for all connection template types.
func (mtp *EmbeddedTemplateProvider) GetAllConnections() (map[Connection][]byte, error) {
	result := make(map[Connection][]byte)
	for _, conn := range ConnectionValues() {
		content, err := mtp.GetConnection(conn)
		if err != nil {
			return nil, fmt.Errorf("%s:%w", conn.String(), err)
		}
		result[conn] = content
	}
	return result, nil
}

// GetValidator returns content of the template file correspond to the provided validator template type.
func (mtp *EmbeddedTemplateProvider) GetValidator(validator Validator) ([]byte, error) {
	return mtp.format(validator.String())
}

// GetAllValidators returns content of the template files for all validator template types.
func (mtp *EmbeddedTemplateProvider) GetAllValidators() (map[Validator][]byte, error) {
	result := make(map[Validator][]byte)
	for _, val := range ValidatorValues() {
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
