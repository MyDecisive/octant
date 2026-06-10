// Package config contains global configurations.
package config

import (
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

// ConfigPathEnvVarName contains the environment variable name
// of the env var that will contain the config file path.
const (
	ConfigPathEnvVarName = "OCTANT_CONFIG_PATH"
	namespaceFilePath    = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// Environment defines the possible environments octant can be running in.
//
//go:generate enumer -type=Environment -text
type Environment int // nolint: recvcheck // the methods are generated

const (
	Dev Environment = iota
	CI
	Prod
)

// Configuration represents the global configurations for octant.
type Configuration struct {
	Port               uint16      `yaml:"port" env:"OCTANT_PORT" env-default:"5678"`
	Env                Environment `yaml:"env" env:"OCTANT_ENV" env-default:"dev"`
	CurrentNamespace   string      `yaml:"currentNamespace" env:"POD_NAMESPACE"`
	ServiceAccountName string      `yaml:"serviceAccountName" env:"OCTANT_SERVICE_ACCOUNT" env-default:"octant"`
	// DefaultTimeout (in seconds) controls HTTP client timeout.
	DefaultTimeout int     `yaml:"defaultTimeout" env:"OCTANT_DEFAULT_TIMEOUT" env-default:"5"`
	Budget         Budget  `yaml:"budget"`
	Install        Install `yaml:"install"`
	Metrics        Metrics `yaml:"metrics"`
}

type Install struct {
	MdaiInstallTimeout               int    `yaml:"mdaiInstallTimeout" env:"MDAI_INSTALL_TIMEOUT" env-default:"90"`
	MdaiInstallPollingIntervalMillis int    `yaml:"mdaiInstallPollingIntervalMillis" env:"MDAI_INSTALL_POLLING_INTERVAL_MILLIS" env-default:"3000"` // nolint:lll
	MdaiValidatorVersion             string `yaml:"mdaiValidatorVersion" env:"MDAI_VALIDATOR_VERSION" env-default:"0.1.3"`
	CerManagerVersion                string `yaml:"certManagerVersion" env:"CERT_MANAGER_VERSION" env-default:"v1.19.1"`
	CerManagerNamespace              string `yaml:"certManagerNamespace" env:"CERT_MANAGER_NAMESPACE" env-default:"cert-manager"` // nolint:lll
}

// Budget contains configuration specifically for budget applet.
type Budget struct {
	MDAIGatewayURLOverride string `yaml:"mdaiGatewayUrlOverride" env:"OCTANT_MDAI_GATEWAY_URL"`
	DefaultMDAIGatewayName string `yaml:"defaultMdaiGateway" env:"OCTANT_DEFAULT_MDAI_GATEWAY" env-default:"mdai-gateway"`
	// FilterSettingUpdateTimeout (in seconds) controls how long
	// Octant waits for filter setting update to be applied.
	FilterSettingUpdateTimeout int `yaml:"filterSettingUpdateTimeout" env:"OCTANT_FILTER_SETTING_UPDATE_TIMEOUT" env-default:"60"` // nolint:lll
	// FilterSettingUpdateInterval (in seconds) controls how often
	//  Octant check if the filter setting update have been applied or not.
	FilterSettingUpdateInterval int `yaml:"filterSettingUpdateInterval" env:"OCTANT_FILTER_SETTING_UPDATE_INTERVAL" env-default:"1"` // nolint:lll

	DefaultLogSamplingRatio   uint32 `env:"OCTANT_DEFAULT_LOG_SAMPLING_RATIO" env-default:"100"`   // nolint:lll
	DefaultLogIncludeErr      bool   `env:"OCTANT_DEFAULT_LOG_INCLUDE_ERR" env-default:"true"`     // nolint:lll
	DefaultTraceSamplingRatio uint32 `env:"OCTANT_DEFAULT_TRACE_SAMPLING_RATIO" env-default:"100"` // nolint:lll
	DefaultTraceIncludeErr    bool   `env:"OCTANT_DEFAULT_TRACE_INCLUDE_ERR" env-default:"true"`   // nolint:lll

	DefaultLogCostRate   float64 `env:"OCTANT_DEFAULT_LOG_COST_RATE" env-default:"2.50"`
	DefaultTraceCostRate float64 `env:"OCTANT_DEFAULT_TRACE_COST_RATE" env-default:"2.50"`

	GreptimeDBURLOverride string `yaml:"greptimedbUrlOverride" env:"OCTANT_GREPTIMEDB_URL"`
	DefaultGreptimeDBName string `yaml:"defaultGreptimedb" env:"OCTANT_DEFAULT_GREPTIMEDB" env-default:"mdai-greptimedb"` // nolint:lll
	// GreptimeDBMaxConnTimeout (in minutes) configures greptime DB max connection timeout.
	GreptimeDBMaxConnTimeout int `yaml:"greptimedbMaxConnTimeout" env:"OCTANT_GREPTIMEDB_MAX_CONN_TIMEOUT" env-default:"3"` // nolint:lll
	GreptimeDBMaxConn        int `yaml:"greptimedbMaxConn" env:"OCTANT_GREPTIMEDB_MAX_CONN" env-default:"10"`               // nolint:lll
}

type Metrics struct {
	PrometheusURLOverride string `yaml:"prometheusUrlOverride" env:"PROMETHEUS_URL"`
	PrometheusPort        int    `yaml:"prometheusPort" env:"PROMETHEUS_PORT" env-default:"9090"`
	PrometheusServiceName string `yaml:"prometheusServiceName" env:"PROMETHEUS_SERVICE_NAME" env-default:"prometheus-operated"` // nolint:lll
}

// Read will read configuration from file first, if `ConfigPathEnvVarName` is set,
// then overrides with value from environment variables;
// otherwise, it will read config purely from environment variables.
func Read() (*Configuration, error) {
	var configuration Configuration
	configPath := strings.TrimSpace(os.Getenv(ConfigPathEnvVarName))
	if configPath == "" {
		if err := cleanenv.ReadEnv(&configuration); err != nil {
			return nil, err
		}
	} else {
		if err := cleanenv.ReadConfig(configPath, &configuration); err != nil {
			return nil, err
		}
	}

	if configuration.CurrentNamespace == "" {
		if data, err := os.ReadFile(namespaceFilePath); err == nil {
			configuration.CurrentNamespace = strings.TrimSpace(string(data))
		}
	}
	return &configuration, nil
}
