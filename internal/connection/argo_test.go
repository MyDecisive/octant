package connection

import (
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func getManifestKinds(manifests []string) mapset.Set[string] {
	kinds := mapset.NewSetWithSize[string](len(manifests))
	for _, m := range manifests {
		var obj struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal([]byte(m), &obj); err != nil || obj.Kind == "" {
			return nil
		}
		kinds.Add(obj.Kind)
	}
	return kinds
}

func connectionSyncManifestsMatcher(manifests []string) bool {
	kinds := getManifestKinds(manifests)
	for _, want := range []string{"Role", "RoleBinding", "Secret", "MdaiHub", "MdaiObserver", "OpenTelemetryCollector"} {
		if !kinds.Contains(want) {
			return false
		}
	}
	return true
}

func validatorSyncManifestsMatcher(manifests []string) bool {
	kinds := getManifestKinds(manifests)
	return kinds.Cardinality() == 1 && kinds.Contains("TelemetryValidation")
}

func TestDeleteArgoApp(t *testing.T) {
	t.Parallel()

	testConfig := &config.Configuration{
		Env: config.Dev,
	}
	ocd := OctantConnectionData{Deployment: &Deployment{IntegrationName: "coolIntegration"}}
	integrationData := &integration.ArgoCDIntegrationData{
		APIUrl:       "http://argo.com",
		AccountToken: "abc123",
	}

	t.Run("unknown error retrieving argo integration", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().GetIntegrationByName(mock.Anything, "coolIntegration").Return(nil, assert.AnError).Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, nil, nil, testConfig, nil, nil)
		require.Error(t, oc.deleteArgoApp(t.Context(), zaptest.NewLogger(t), "mdai", ocd))
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().GetIntegrationByName(mock.Anything, "coolIntegration").Return(integrationData, nil).Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().DeleteArgoApp(mock.Anything, mock.Anything, mock.MatchedBy(func(opts *apiclient.ClientOptions) bool {
			return opts.ServerAddr == "http://argo.com" && opts.AuthToken == "abc123"
		}), "mdai").Return(nil).Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, nil, nil, testConfig, mockArgoClient, nil)
		require.NoError(t, oc.deleteArgoApp(t.Context(), zaptest.NewLogger(t), "mdai", ocd))
	})
}

func TestDeleteValidatorResource(t *testing.T) {
	t.Parallel()

	templateData := &ArgoConnectionTemplateData{
		AppName:          "coolIntegration",
		Namespace:        "mdai",
		ServiceAccount:   "coolServiceAccount",
		CurrentNamespace: "default",
		ConnectionData: OctantConnectionData{
			Deployment: &Deployment{
				Type:            ArgoSideloadDeploymentType,
				IntegrationName: "coolIntegration",
			},
			SourceType: "datadog",
			Destinations: []OctantConnectionDestination{
				{
					DestinationType: "datadog",
					IntegrationName: "coolIntegration",
				},
			},
			TelemetryTypes: []telemetry.MLT{
				telemetry.Logs,
				telemetry.Traces,
			},
			MdaiNamespace: "mdai",
		},
		IsArgoSideload: true,
	}

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(argoIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "coolIntegration", mock.MatchedBy(connectionSyncManifestsMatcher), true).
			Return(nil).
			Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, nil, nil, testConfig, mockArgoClient, NewConnectionManifestGenerator(testConfig))
		require.NoError(t, oc.deleteValidatorResource(t.Context(), zaptest.NewLogger(t), "coolIntegration", templateData))
	})
}

func TestSideloadConnectionApp(t *testing.T) {
	t.Parallel()

	generator := NewConnectionManifestGenerator(testConfig)
	ocd := OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "coolIntegration"},
		Destinations: []OctantConnectionDestination{
			{
				DestinationType: "datadog",
				IntegrationName: "coolIntegration",
			},
		},
	}

	t.Run("error creating template data", func(t *testing.T) {
		t.Parallel()

		connectionData := OctantConnectionData{
			Deployment: &Deployment{IntegrationName: "coolIntegration"},
			Destinations: []OctantConnectionDestination{
				{
					DestinationType: "datadog",
					IntegrationName: "coolIntegration",
				},
				{
					DestinationType: "datadog",
					IntegrationName: "otherCoolIntegration", // multiple destinations will fail creating the template
				},
			},
		}
		oc := NewOctantConnection(nil, nil, nil, nil, testConfig, nil, generator)
		require.ErrorContains(t,
			oc.sideloadConnectionApp(t.Context(), zaptest.NewLogger(t), "mdai", connectionData),
			"pushing argo application with multiple destinations is currently unsupported",
		)
	})

	t.Run("error getting argo integration data", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(nil, assert.AnError).
			Once()

		mockDatadogIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadogIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(ddIntegrationData, nil).
			Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, mockDatadogIntegration, nil, testConfig, nil, nil)
		require.Error(t, oc.sideloadConnectionApp(t.Context(), zaptest.NewLogger(t), "mdai", ocd))
	})

	t.Run("error pushing argo app", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(argoIntegrationData, nil).
			Once()

		mockDatadogIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadogIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(ddIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(assert.AnError).
			Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, mockDatadogIntegration, nil, testConfig, mockArgoClient, generator)
		require.Error(t, oc.sideloadConnectionApp(t.Context(), zaptest.NewLogger(t), "mdai", ocd))
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(argoIntegrationData, nil).
			Once()

		mockDatadogIntegration := integrationmock.NewMockIntegration[integration.DataDogIntegrationData](t)
		mockDatadogIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(ddIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Once()
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "coolIntegration", mock.MatchedBy(connectionSyncManifestsMatcher), false).
			Return(nil).
			Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, mockDatadogIntegration, nil, testConfig, mockArgoClient, generator)
		require.NoError(t, oc.sideloadConnectionApp(t.Context(), zaptest.NewLogger(t), "coolIntegration", ocd))
	})
}

func TestSideloadValidatorForConnection(t *testing.T) {
	t.Parallel()

	generator := NewConnectionManifestGenerator(testConfig)

	t.Run("error getting argo integration data", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(nil, assert.AnError).
			Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, nil, nil, testConfig, nil, nil)
		validatorRunID, err := oc.sideloadValidatorForConnection(t.Context(), zaptest.NewLogger(t), "coolIntegration", defaultNamespace)
		require.Error(t, err)
		require.Empty(t, validatorRunID)
	})

	t.Run("error syncing argo app", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(argoIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "coolIntegration", mock.MatchedBy(validatorSyncManifestsMatcher), false).
			Return(assert.AnError).
			Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, nil, nil, testConfig, mockArgoClient, generator)
		validatorRunID, err := oc.sideloadValidatorForConnection(t.Context(), zaptest.NewLogger(t), "coolIntegration", defaultNamespace)
		require.Error(t, err)
		require.Empty(t, validatorRunID)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		mockArgoIntegration := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgoIntegration.EXPECT().
			GetIntegrationByName(mock.Anything, "coolIntegration").
			Return(argoIntegrationData, nil).
			Once()

		mockArgoClient := argocdmock.NewMockAPIClient(t)
		mockArgoClient.EXPECT().
			SyncApplication(mock.Anything, mock.Anything, mock.Anything, "coolIntegration", mock.MatchedBy(validatorSyncManifestsMatcher), false).
			Return(nil).
			Once()

		oc := NewOctantConnection(nil, mockArgoIntegration, nil, nil, testConfig, mockArgoClient, generator)
		validatorRunID, err := oc.sideloadValidatorForConnection(t.Context(), zaptest.NewLogger(t), "coolIntegration", defaultNamespace)
		require.NoError(t, err)
		require.NotEmpty(t, validatorRunID)
	})
}
