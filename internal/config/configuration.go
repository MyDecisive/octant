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
	Env              Environment `yaml:"env" env:"OCTANT_ENV" env-default:"dev"`
	CurrentNamespace string      `yaml:"currentNamespace" env:"POD_NAMESPACE"`
	// DefaultTimeout (in seconds) controls HTTP client timeout.
	DefaultTimeout int    `yaml:"defaultTimeout" env:"OCTANT_DEFAULT_TIMEOUT" env-default:"5"`
	RPC            RPC    `yaml:"rpc"`
	Budget         Budget `yaml:"budget"`
}

// RPC contains configuration for RPC related code.
type RPC struct {
	Port uint16 `yaml:"port" env:"OCTANT_RPC_PORT" env-default:"50051"`
}

// Budget contains configuration specifically for budget applet.
type Budget struct {
	DefaultMDAIGatewayName string `yaml:"defaultMdaiGateway" env:"OCTANT_DEFAULT_MDAI_GATEWAY" env-default:"mdai-gateway"`
	// FilterSettingUpdateTimeout (in seconds) controls how long
	// Octant waits for filter setting update to be applied.
	FilterSettingUpdateTimeout int `yaml:"filterSettingUpdateTimeout" env:"OCTANT_FILTER_SETTING_UPDATE_TIMEOUT" env-default:"60"` // nolint:lll
	// FilterSettingUpdateInterval (in seconds) controls how often
	//  Octant check if the filter setting update have been applied or not.
	FilterSettingUpdateInterval int `yaml:"filterSettingUpdateInterval" env:"OCTANT_FILTER_SETTING_UPDATE_INTERVAL" env-default:"1"` // nolint:lll
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
		configuration.CurrentNamespace = getCurrentNamespace()
	}
	return &configuration, nil
}

func getCurrentNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	if data, err := os.ReadFile(namespaceFilePath); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}
	return ""
}
