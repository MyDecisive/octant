package manifestdata

import (
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDataMapper_AppTemplateData(t *testing.T) {
	t.Parallel()

	version := faker.Word()
	conn := faker.Word()
	namespace := faker.Word()

	conf := &config.Configuration{
		Install: config.Install{
			CertManagerVersion:   faker.Word(),
			CertManagerNamespace: faker.Word(),
			ArgoCDNamespace:      faker.Word(),
		},
	}

	cases := []struct {
		app      App
		expected AppTemplateData
	}{
		{
			CERT,
			AppTemplateData{
				Version:         conf.Install.CertManagerVersion,
				Namespace:       conf.Install.CertManagerNamespace,
				ArgoCDNamespace: conf.Install.ArgoCDNamespace,
			},
		},
		{
			MDAI,
			AppTemplateData{
				Version:         version,
				Namespace:       namespace,
				ArgoCDNamespace: conf.Install.ArgoCDNamespace,
			},
		},
		{
			CONNECTION,
			AppTemplateData{
				Name:            conn,
				Namespace:       namespace,
				ArgoCDNamespace: conf.Install.ArgoCDNamespace,
			},
		},
		{
			VALIDATOR,
			AppTemplateData{
				Name:            conn,
				Namespace:       namespace,
				ArgoCDNamespace: conf.Install.ArgoCDNamespace,
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.app.String(), func(t *testing.T) {
			t.Parallel()
			target := NewDataMapper(conf, nil)
			actual := target.AppTemplateData(tt.app, version, conn, namespace)

			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestDataMapper_ConnectionTemplateData(t *testing.T) {
	t.Parallel()
	conf := &config.Configuration{
		CurrentNamespace:   faker.Word(),
		ServiceAccountName: faker.Word(),
		Budget: config.Budget{
			DefaultLogSamplingRatio:   1,
			DefaultLogIncludeErr:      true,
			DefaultTraceSamplingRatio: 2,
			DefaultTraceIncludeErr:    false,
		},
	}

	datadog := integration.DataDogIntegrationData{
		APIKey:   faker.Word(),
		SiteHost: faker.Word(),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		input := ConnectionInput{
			ConnectionName:            faker.Word(),
			DeploymentIntegrationName: faker.Word(),
			Namespace:                 faker.Word(),
			TelemetryTypes:            []telemetry.MLT{telemetry.Logs},
			Destinations: []Destination{{
				Type:            DATADOG,
				IntegrationName: faker.Word(),
			}},
		}

		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadog.EXPECT().GetIntegrationByName(mock.Anything, input.Destinations[0].IntegrationName).Return(&datadog, nil).Once()

		target := NewDataMapper(conf, mockDatadog)

		actual, err := target.ConnectionTemplateData(t.Context(), input)
		require.NoError(t, err)

		assert.Equal(t, input.ConnectionName, actual.Name)
		assert.Equal(t, input.Namespace, actual.Namespace)
		assert.Equal(t, input.TelemetryTypes, actual.TelemetryTypes)
		assert.Equal(t, &datadog, actual.DatadogIntegrationData)
		assert.Equal(t, conf.CurrentNamespace, actual.CurrentNamespace)
		assert.Equal(t, conf.ServiceAccountName, actual.ServiceAccount)
		assert.Equal(t, "1", actual.DefaultLogRatio)
		assert.True(t, actual.DefaultLogPersistErr)
		assert.Equal(t, "2", actual.DefaultTraceRatio)
		assert.False(t, actual.DefaultTracePersistErr)
	})

	t.Run("success exported", func(t *testing.T) {
		t.Parallel()

		input := ConnectionInput{
			ConnectionName:            faker.Word(),
			DeploymentIntegrationName: faker.Word(),
			Namespace:                 faker.Word(),
			TelemetryTypes:            []telemetry.MLT{telemetry.Logs},
			Destinations: []Destination{{
				Type:            DATADOG,
				IntegrationName: faker.Word(),
			}},
			Exported: true,
		}

		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		target := NewDataMapper(conf, mockDatadog)

		actual, err := target.ConnectionTemplateData(t.Context(), input)
		require.NoError(t, err)

		assert.Equal(t, input.ConnectionName, actual.Name)
		assert.Equal(t, input.Namespace, actual.Namespace)
		assert.Equal(t, input.TelemetryTypes, actual.TelemetryTypes)
		assert.Equal(t, "<YOUR_API_KEY>", actual.DatadogIntegrationData.APIKey)
		assert.Equal(t, "<YOUR_DD_SITE_HOST>", actual.DatadogIntegrationData.SiteHost)
		assert.Equal(t, conf.CurrentNamespace, actual.CurrentNamespace)
		assert.Equal(t, conf.ServiceAccountName, actual.ServiceAccount)
		assert.Equal(t, "1", actual.DefaultLogRatio)
		assert.True(t, actual.DefaultLogPersistErr)
		assert.Equal(t, "2", actual.DefaultTraceRatio)
		assert.False(t, actual.DefaultTracePersistErr)
	})

	t.Run("err datadog", func(t *testing.T) {
		t.Parallel()

		input := ConnectionInput{
			ConnectionName:            faker.Word(),
			DeploymentIntegrationName: faker.Word(),
			Namespace:                 faker.Word(),
			TelemetryTypes:            []telemetry.MLT{telemetry.Logs},
			Destinations: []Destination{{
				Type:            DATADOG,
				IntegrationName: faker.Word(),
			}},
		}

		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadog.EXPECT().GetIntegrationByName(mock.Anything, input.Destinations[0].IntegrationName).Return(nil, assert.AnError).Once()

		target := NewDataMapper(conf, mockDatadog)

		actual, err := target.ConnectionTemplateData(t.Context(), input)
		assert.Nil(t, actual)
		require.ErrorIs(t, err, ErrIntegration)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("err no datadog", func(t *testing.T) {
		t.Parallel()

		input := ConnectionInput{
			ConnectionName:            faker.Word(),
			DeploymentIntegrationName: faker.Word(),
			Namespace:                 faker.Word(),
			TelemetryTypes:            []telemetry.MLT{telemetry.Logs},
			Destinations: []Destination{{
				Type:            DATADOG,
				IntegrationName: faker.Word(),
			}},
		}

		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadog.EXPECT().GetIntegrationByName(mock.Anything, input.Destinations[0].IntegrationName).Return(nil, nil).Once()

		target := NewDataMapper(conf, mockDatadog)

		actual, err := target.ConnectionTemplateData(t.Context(), input)
		assert.Nil(t, actual)
		require.ErrorIs(t, err, ErrIntegration)
		require.ErrorContains(t, err, "not found")
	})

	t.Run("err invalid destination type", func(t *testing.T) {
		t.Parallel()

		input := ConnectionInput{
			ConnectionName:            faker.Word(),
			DeploymentIntegrationName: faker.Word(),
			Namespace:                 faker.Word(),
			TelemetryTypes:            []telemetry.MLT{telemetry.Logs},
			Destinations: []Destination{{
				Type:            DestinationType(-1),
				IntegrationName: faker.Word(),
			}},
		}

		mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

		target := NewDataMapper(conf, mockDatadog)

		actual, err := target.ConnectionTemplateData(t.Context(), input)
		assert.Nil(t, actual)
		require.ErrorIs(t, err, ErrUnknown)
	})
}

func TestDataMapper_ValidatorTemplateData(t *testing.T) {
	t.Parallel()
	conf := &config.Configuration{
		Install: config.Install{
			MdaiValidatorVersion: faker.Word(),
		},
	}

	input := ValidatorInput{
		ConnectionName:            faker.Word(),
		DeploymentIntegrationName: faker.Word(),
		Namespace:                 faker.Word(),
		RunID:                     faker.Word(),
	}

	mockDatadog := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)

	target := NewDataMapper(conf, mockDatadog)

	actual := target.ValidatorTemplateData(input)

	assert.Equal(t, input.ConnectionName, actual.Name)
	assert.Equal(t, input.Namespace, actual.Namespace)
	assert.Equal(t, conf.Install.MdaiValidatorVersion, actual.Version)
	assert.Equal(t, input.RunID, actual.RunID)
}
