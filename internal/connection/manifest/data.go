package manifest

import (
	"strconv"

	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/telemetry"
)

// AppTemplateData will be passed to the renderer as the data input
// for argocd app template.
type AppTemplateData struct {
	Name string // connection name
	// Not applicable for Validator and Connection.
	// For Cert, this value come from config.
	// For MDAI, this value provided by user.
	Version string
	// For Cert, this value come from config.
	Namespace string
}

// ConnectionTemplateData will be passed to the renderer as the data input
// for connection manifest template.
type ConnectionTemplateData struct {
	IsArgoSideload         bool
	Name                   string // connection name
	Namespace              string
	CurrentNamespace       string // from config
	TelemetryTypes         []telemetry.MLT
	DatadogIntegrationData *integration.DataDogIntegrationData
	DefaultLogRatio        string // from config
	DefaultLogPersistErr   bool   // from config
	DefaultTraceRatio      string // from config
	DefaultTracePersistErr bool   // from config
}

// ValidatorTemplateData will be passed to the renderer as the data input
// for validator manifest template.
type ValidatorTemplateData struct {
	IsArgoSideload bool
	Name           string // connection name
	Namespace      string
	Version        string // from config
	RunID          string
}

// getConnectionTemplateData generates ConnectionTemplateData using the given input and config.
func getConnectionTemplateData(conf *config.Configuration, input ConnectionInput) ConnectionTemplateData {
	return ConnectionTemplateData{
		IsArgoSideload:         input.IsArgoSideload,
		Name:                   input.Name,
		Namespace:              input.Namespace,
		TelemetryTypes:         input.TelemetryTypes,
		DatadogIntegrationData: input.DatadogIntegrationData,
		CurrentNamespace:       conf.CurrentNamespace,
		DefaultLogRatio:        strconv.FormatUint(uint64(conf.Budget.DefaultLogSamplingRatio), 10),
		DefaultLogPersistErr:   conf.Budget.DefaultLogIncludeErr,
		DefaultTraceRatio:      strconv.FormatUint(uint64(conf.Budget.DefaultTraceSamplingRatio), 10),
		DefaultTracePersistErr: conf.Budget.DefaultTraceIncludeErr,
	}
}

// getValidatorTemplateData generates ValidatorTemplateData using the given input and config.
func getValidatorTemplateData(conf *config.Configuration, input ValidatorInput) ValidatorTemplateData {
	return ValidatorTemplateData{
		IsArgoSideload: input.IsArgoSideload,
		Name:           input.Name,
		Namespace:      input.Namespace,
		Version:        conf.Install.MdaiValidatorVersion,
		RunID:          input.RunID,
	}
}

// GetCertAppTemplateData generates AppTemplateData for app type Cert using config.
func GetCertAppTemplateData(conf *config.Configuration) AppTemplateData {
	return AppTemplateData{
		Version:   conf.Install.CerManagerVersion,
		Namespace: conf.Install.CerManagerNamespace,
	}
}

// GetAppTemplateData generates GetAppTemplateData corresponds to the given app type.
func GetAppTemplateData(
	app App,
	conf *config.Configuration,
	mdaiVer string,
	connectionName string,
	namespace string,
) AppTemplateData {
	switch app {
	case CERT:
		return GetCertAppTemplateData(conf)
	case MDAI:
		return AppTemplateData{
			Version:   mdaiVer,
			Namespace: namespace,
		}
	default:
		return AppTemplateData{
			Name:      connectionName,
			Namespace: namespace,
		}
	}
}
