package manifest

import (
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/config"
	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	"github.com/mydecisive/octant/internal/integration"
	argocdmock "github.com/mydecisive/octant/internal/mock/argocd"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	manifestmock "github.com/mydecisive/octant/internal/mock/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestArgoCDManager_GetAppName(t *testing.T) {
	t.Parallel()

	for _, tt := range manifestdata.AppValues() {
		t.Run(tt.String(), func(t *testing.T) {
			t.Parallel()

			target := NewArgoCDManager(nil, nil, nil, nil)
			expected := target.getAppName(tt, "{{ .Name }}")

			actual, err := templates.ReadFile("template/" + tt.String() + ".yaml.tmpl")
			require.NoError(t, err)
			assert.Contains(t, string(actual), expected)
		})
	}
}

func TestArgoCDManager_Unload(t *testing.T) {
	t.Parallel()

	conf := config.Configuration{
		Env: config.Dev,
	}
	app := []manifestdata.App{
		manifestdata.MDAI,
	}
	input := manifestdata.ManagerInput{
		Logger:                    zaptest.NewLogger(t),
		DeploymentIntegrationName: faker.Word(),
		ConnectionName:            faker.Word(),
	}
	var argo integration.ArgoCDIntegrationData
	require.NoError(t, faker.FakeData(&argo))
	clientInputMatch := func(in argocd.Input) bool {
		return in.AppName == "mdai"
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().DeleteArgoApp(mock.Anything, mock.MatchedBy(clientInputMatch)).Return(nil).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, nil)
		err := target.Unload(t.Context(), input, app)
		assert.NoError(t, err)
	})

	t.Run("err integration", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(nil, assert.AnError).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, nil)
		err := target.Unload(t.Context(), input, app)
		assert.ErrorIs(t, err, manifestdata.ErrIntegration)
	})

	t.Run("err no argo", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(nil, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, nil)
		err := target.Unload(t.Context(), input, app)
		assert.ErrorContains(t, err, "no argocd")
	})

	t.Run("err delete", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().DeleteArgoApp(mock.Anything, mock.MatchedBy(clientInputMatch)).Return(assert.AnError).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, nil)
		err := target.Unload(t.Context(), input, app)
		assert.Error(t, err)
	})
}

func TestArgoCDManager_LoadCertManager(t *testing.T) {
	t.Parallel()

	conf := config.Configuration{
		Env: config.Dev,
	}
	input := manifestdata.ManagerInput{
		Logger:                    zaptest.NewLogger(t),
		DeploymentIntegrationName: faker.Word(),
		ConnectionName:            faker.Word(),
	}
	var data manifestdata.AppTemplateData
	require.NoError(t, faker.FakeData(&data))

	var argo integration.ArgoCDIntegrationData
	require.NoError(t, faker.FakeData(&argo))

	appManifest := [][]byte{[]byte(`{"spec":{"project":"default"}}`)}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.CERT, data, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadCertManager(t.Context(), input, data)
		assert.NoError(t, err)
	})

	t.Run("err integration", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(nil, assert.AnError).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadCertManager(t.Context(), input, data)
		assert.ErrorIs(t, err, manifestdata.ErrIntegration)
	})

	t.Run("err get manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.CERT, data, manifestdata.JSON).Return(nil, manifestdata.ErrUnknown).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadCertManager(t.Context(), input, data)
		assert.ErrorIs(t, err, manifestdata.ErrUnknown)
	})

	t.Run("err invalid manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.CERT, data, manifestdata.JSON).Return([][]byte{[]byte("a")}, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadCertManager(t.Context(), input, data)
		assert.ErrorIs(t, err, ErrEmpty)
	})

	t.Run("err push", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.CERT, data, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadCertManager(t.Context(), input, data)
		assert.ErrorIs(t, err, ErrPushApp)
	})
}

func TestArgoCDManager_LoadMDAI(t *testing.T) {
	t.Parallel()

	conf := config.Configuration{
		Env: config.Dev,
	}
	input := manifestdata.ManagerInput{
		Logger:                    zaptest.NewLogger(t),
		DeploymentIntegrationName: faker.Word(),
		ConnectionName:            faker.Word(),
	}
	var data manifestdata.AppTemplateData
	require.NoError(t, faker.FakeData(&data))

	var argo integration.ArgoCDIntegrationData
	require.NoError(t, faker.FakeData(&argo))

	appManifest := [][]byte{[]byte(`{"spec":{"project":"default"}}`)}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.MDAI, data, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadMDAI(t.Context(), input, data)
		assert.NoError(t, err)
	})

	t.Run("err integration", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(nil, assert.AnError).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadMDAI(t.Context(), input, data)
		assert.ErrorIs(t, err, manifestdata.ErrIntegration)
	})

	t.Run("err get manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.MDAI, data, manifestdata.JSON).Return(nil, manifestdata.ErrUnknown).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadMDAI(t.Context(), input, data)
		assert.ErrorIs(t, err, manifestdata.ErrUnknown)
	})

	t.Run("err invalid manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.MDAI, data, manifestdata.JSON).Return([][]byte{[]byte("a")}, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadMDAI(t.Context(), input, data)
		assert.ErrorIs(t, err, ErrEmpty)
	})

	t.Run("err push", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.MDAI, data, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadMDAI(t.Context(), input, data)
		assert.ErrorIs(t, err, ErrPushApp)
	})
}

func TestArgoCDManager_LoadConnection(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	conf := config.Configuration{
		Env: config.Dev,
	}
	var input manifestdata.ConnectionInput
	require.NoError(t, faker.FakeData(&input))
	var argo integration.ArgoCDIntegrationData
	require.NoError(t, faker.FakeData(&argo))

	appManifest := [][]byte{[]byte(`{"spec":{"project":"default"}}`)}
	rawManifest := faker.Word()
	manifests := map[string][]byte{
		faker.Word(): []byte(rawManifest),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Connections(mock.Anything, input, manifestdata.JSON).Return(manifests, nil).Once()
		mockGenerator.EXPECT().App(manifestdata.CONNECTION, mock.Anything, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockClient.EXPECT().SyncApplication(mock.Anything, mock.Anything, []string{rawManifest}, false).Return(nil).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadConnection(t.Context(), logger, input)
		assert.NoError(t, err)
	})

	t.Run("no connection manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Connections(mock.Anything, input, manifestdata.JSON).Return(map[string][]byte{}, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadConnection(t.Context(), logger, input)
		assert.NoError(t, err)
	})

	t.Run("err connection manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Connections(mock.Anything, input, manifestdata.JSON).Return(nil, manifestdata.ErrUnknown).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadConnection(t.Context(), logger, input)
		assert.ErrorIs(t, err, manifestdata.ErrUnknown)
	})

	t.Run("err integration", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(nil, assert.AnError).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Connections(mock.Anything, input, manifestdata.JSON).Return(manifests, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadConnection(t.Context(), logger, input)
		assert.ErrorIs(t, err, manifestdata.ErrIntegration)
	})

	t.Run("err get app manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.CONNECTION, mock.Anything, manifestdata.JSON).Return(nil, manifestdata.ErrUnknown).Once()
		mockGenerator.EXPECT().Connections(mock.Anything, input, manifestdata.JSON).Return(manifests, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadConnection(t.Context(), logger, input)
		assert.ErrorIs(t, err, manifestdata.ErrUnknown)
	})

	t.Run("err push app", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Connections(mock.Anything, input, manifestdata.JSON).Return(manifests, nil).Once()
		mockGenerator.EXPECT().App(manifestdata.CONNECTION, mock.Anything, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadConnection(t.Context(), logger, input)
		assert.ErrorIs(t, err, ErrPushApp)
	})

	t.Run("err push manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Connections(mock.Anything, input, manifestdata.JSON).Return(manifests, nil).Once()
		mockGenerator.EXPECT().App(manifestdata.CONNECTION, mock.Anything, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockClient.EXPECT().SyncApplication(mock.Anything, mock.Anything, []string{rawManifest}, false).Return(assert.AnError).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadConnection(t.Context(), logger, input)
		assert.ErrorIs(t, err, ErrPushManifests)
	})
}

func TestArgoCDManager_LoadValidator(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	conf := config.Configuration{
		Env: config.Dev,
	}
	var input manifestdata.ValidatorInput
	require.NoError(t, faker.FakeData(&input))
	var argo integration.ArgoCDIntegrationData
	require.NoError(t, faker.FakeData(&argo))

	appManifest := [][]byte{[]byte(`{"spec":{"project":"default"}}`)}
	rawManifest := faker.Word()
	manifests := map[string][]byte{
		faker.Word(): []byte(rawManifest),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Validators(input, manifestdata.JSON).Return(manifests, nil).Once()
		mockGenerator.EXPECT().App(manifestdata.VALIDATOR, mock.Anything, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockClient.EXPECT().SyncApplication(mock.Anything, mock.Anything, []string{rawManifest}, false).Return(nil).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadValidator(t.Context(), logger, input)
		assert.NoError(t, err)
	})

	t.Run("no connection manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Validators(input, manifestdata.JSON).Return(map[string][]byte{}, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadValidator(t.Context(), logger, input)
		assert.NoError(t, err)
	})

	t.Run("err connection manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Validators(input, manifestdata.JSON).Return(nil, manifestdata.ErrUnknown).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadValidator(t.Context(), logger, input)
		assert.ErrorIs(t, err, manifestdata.ErrUnknown)
	})

	t.Run("err integration", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(nil, assert.AnError).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Validators(input, manifestdata.JSON).Return(manifests, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadValidator(t.Context(), logger, input)
		assert.ErrorIs(t, err, manifestdata.ErrIntegration)
	})

	t.Run("err get app manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().App(manifestdata.VALIDATOR, mock.Anything, manifestdata.JSON).Return(nil, manifestdata.ErrUnknown).Once()
		mockGenerator.EXPECT().Validators(input, manifestdata.JSON).Return(manifests, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadValidator(t.Context(), logger, input)
		assert.ErrorIs(t, err, manifestdata.ErrUnknown)
	})

	t.Run("err push app", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Validators(input, manifestdata.JSON).Return(manifests, nil).Once()
		mockGenerator.EXPECT().App(manifestdata.VALIDATOR, mock.Anything, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadValidator(t.Context(), logger, input)
		assert.ErrorIs(t, err, ErrPushApp)
	})

	t.Run("err push manifest", func(t *testing.T) {
		t.Parallel()

		mockArgo := integrationmock.NewMockIntegration[integration.ArgoCDIntegrationData](t)
		mockArgo.EXPECT().GetIntegrationByName(mock.Anything, input.DeploymentIntegrationName).Return(&argo, nil).Once()
		mockGenerator := manifestmock.NewMockGenerator(t)
		mockGenerator.EXPECT().Validators(input, manifestdata.JSON).Return(manifests, nil).Once()
		mockGenerator.EXPECT().App(manifestdata.VALIDATOR, mock.Anything, manifestdata.JSON).Return(appManifest, nil).Once()
		mockClient := argocdmock.NewMockAPIClient(t)
		mockClient.EXPECT().PushArgoApp(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockClient.EXPECT().SyncApplication(mock.Anything, mock.Anything, []string{rawManifest}, false).Return(assert.AnError).Once()

		target := NewArgoCDManager(&conf, mockArgo, mockClient, mockGenerator)
		err := target.LoadValidator(t.Context(), logger, input)
		assert.ErrorIs(t, err, ErrPushManifests)
	})
}
