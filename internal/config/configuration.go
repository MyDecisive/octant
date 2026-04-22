// Package config contains global configurations.
package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"os"
	"strings"
)

// ConfigPathEnvVarName contains the environment variable name
// of the env var that will contain the config file path.
const ConfigPathEnvVarName = "OCTANT_CONFIG_PATH"

// Environment defines the possible environments octant can be running in.
//
//go:generate enumer -type=Environment -text
type Environment int

const (
	Dev Environment = iota
	CI
	Prod
)

// Configuration represents the global configurations for octant.
type Configuration struct {
	Env              Environment `yaml:"env" env:"OCTANT_ENV" env-default:"dev"`
	RPC              RPC         `yaml:"rpc"`
	InstallNamespace string      `yaml:"install_namespace" env:"OCTANT_INSTALL_NAMESPACE" env-default:"default"`
}

// RPC contains configuration for RPC related code.
type RPC struct {
	Port uint16 `yaml:"port" env:"OCTANT_RPC_PORT" env-default:"50051"`
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
	return &configuration, nil
}
