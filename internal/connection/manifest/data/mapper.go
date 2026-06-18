package manifestdata

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
)

var (
	ErrUnsupported = errors.New("unsupported")
	ErrUnknown     = errors.New("unknown")
	ErrIntegration = errors.New("integration")
)

type Mapper interface {
	// AppTemplateData generates GetAppTemplateData corresponds to the given app type.
	// Note: mdaiVersion is only needed for MDAI app type,
	// and connectionName is only needed for Connection and validator app type.
	// Note: Cert app type does not need anything.
	AppTemplateData(app App, mdaiVersion string, connectionName string, namespace string) AppTemplateData
	// ConnectionTemplateData returns ConnectionTemplateData base on given input.
	ConnectionTemplateData(ctx context.Context, input ConnectionInput) (*ConnectionTemplateData, error)
	// ValidatorTemplateData generates ValidatorTemplateData using the given config.
	ValidatorTemplateData(input ValidatorInput) ValidatorTemplateData
}

// DataMapper implements Mapper.
type DataMapper struct {
	config  *config.Configuration
	datadog integration.Integration[integration.DataDogIntegrationData]
}

// Ensure ManifestGenerator implements Generator.
var _ Mapper = (*DataMapper)(nil)

// NewDataMapper returns a new instance of DataMapper.
func NewDataMapper(
	conf *config.Configuration,
	datadog integration.Integration[integration.DataDogIntegrationData],
) *DataMapper {
	return &DataMapper{
		config:  conf,
		datadog: datadog,
	}
}

// AppTemplateData generates GetAppTemplateData corresponds to the given app type.
// Note: mdaiVersion is only needed for MDAI app type,
// and connectionName is only needed for Connection and validator app type.
// Note: Cert app type does not need anything.
func (dm *DataMapper) AppTemplateData(
	app App,
	mdaiVersion string,
	connectionName string,
	namespace string,
) AppTemplateData {
	switch app {
	case CERT:
		return AppTemplateData{
			Version:         dm.config.Install.CerManagerVersion,
			Namespace:       dm.config.Install.CerManagerNamespace,
			ArgoCDNamespace: dm.config.Install.ArgoCDNamespace,
		}
	case MDAI:
		return AppTemplateData{
			Version:         mdaiVersion,
			Namespace:       namespace,
			ArgoCDNamespace: dm.config.Install.ArgoCDNamespace,
		}
	default:
		return AppTemplateData{
			Name:            connectionName,
			Namespace:       namespace,
			ArgoCDNamespace: dm.config.Install.ArgoCDNamespace,
		}
	}
}

// ConnectionTemplateData returns ConnectionTemplateData base on given input.
func (dm *DataMapper) ConnectionTemplateData(
	ctx context.Context,
	input ConnectionInput,
) (*ConnectionTemplateData, error) {
	datadog := &integration.DataDogIntegrationData{ // nolint:gosec // no, these are not secrets lol
		APIKey: "<YOUR_API_KEY>",
		DDUrl:  "<YOUR_DD_URL>",
	}
	for _, destination := range input.Destinations {
		switch destination.Type {
		case DATADOG:
			if !input.Exported {
				data, err := dm.datadog.GetIntegrationByName(ctx, destination.IntegrationName)
				if err != nil {
					return nil, fmt.Errorf("%w: %w", ErrIntegration, err)
				}
				if data == nil {
					return nil, fmt.Errorf("%w: not found", ErrIntegration)
				}
				datadog = data
			}
		default:
			return nil, fmt.Errorf("%w: destination %s", ErrUnknown, destination)
		}
	}
	return &ConnectionTemplateData{
		Name:                   input.ConnectionName,
		Namespace:              input.Namespace,
		TelemetryTypes:         input.TelemetryTypes,
		DatadogIntegrationData: datadog,
		CurrentNamespace:       dm.config.CurrentNamespace,
		ServiceAccount:         dm.config.ServiceAccountName,
		DefaultLogRatio:        strconv.FormatUint(uint64(dm.config.Budget.DefaultLogSamplingRatio), 10),
		DefaultLogPersistErr:   dm.config.Budget.DefaultLogIncludeErr,
		DefaultTraceRatio:      strconv.FormatUint(uint64(dm.config.Budget.DefaultTraceSamplingRatio), 10),
		DefaultTracePersistErr: dm.config.Budget.DefaultTraceIncludeErr,
	}, nil
}

// ValidatorTemplateData generates ValidatorTemplateData using the given config.
func (dm *DataMapper) ValidatorTemplateData(input ValidatorInput) ValidatorTemplateData {
	return ValidatorTemplateData{
		Name:      input.ConnectionName,
		Namespace: input.Namespace,
		Version:   dm.config.Install.MdaiValidatorVersion,
		RunID:     input.RunID,
	}
}
