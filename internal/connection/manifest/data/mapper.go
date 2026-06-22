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
	// AppTemplateData generates AppTemplateData for the given app type:
	//  - For CERT, this will ignore all parameters and retrieve cert manager version and namespace from config
	//  - For MDAI, this will populate version and namespace using the provided mdaiVersion and namespace
	//  - For all other app types, this will populate name and namespace using the provided connection name and namespace
	//
	// Note: Base on the given app type, the AppTemplateData field(s) unused by the app type will be empty.
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

// AppTemplateData generates AppTemplateData for the given app type:
//   - For CERT, this will ignore all parameters and retrieve cert manager version and namespace from config
//   - For MDAI, this will populate version and namespace using the provided mdaiVersion and namespace
//   - For all other app types, this will populate name and namespace using the provided connection name and namespace
//
// Note: Base on the given app type, the AppTemplateData field(s) unused by the app type will be empty.
func (dm *DataMapper) AppTemplateData(
	app App,
	mdaiVersion string,
	connectionName string,
	namespace string,
) AppTemplateData {
	switch app {
	case CERT:
		return AppTemplateData{
			Version:   dm.config.Install.CertManagerVersion,
			Namespace: dm.config.Install.CertManagerNamespace,
		}
	case MDAI:
		return AppTemplateData{
			Version:   mdaiVersion,
			Namespace: namespace,
		}
	default:
		return AppTemplateData{
			Name:      connectionName,
			Namespace: namespace,
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
			if input.Exported {
				continue
			}
			data, err := dm.datadog.GetIntegrationByName(ctx, destination.IntegrationName)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrIntegration, err)
			}
			if data == nil {
				return nil, fmt.Errorf("%w: not found", ErrIntegration)
			}
			datadog = data
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
