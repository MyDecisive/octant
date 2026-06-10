package manfiestdata

// OutputFormat defines the possible validator app specific templates octant can generate.
//
//go:generate enumer -type=OutputFormat -transform=lower -text
type OutputFormat int // nolint: recvcheck // the methods are generated

const (
	YAML OutputFormat = iota
	JSON
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

// Destination defines the possible Octant connection destination types.
//
//go:generate enumer -type=Destination -transform=lower -text
type Destination int // nolint: recvcheck // the methods are generated

const (
	DATADOG Destination = iota
)
