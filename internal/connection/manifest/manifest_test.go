package manifest

import (
	"testing"

	"github.com/go-faker/faker/v4"
	manifestdata "github.com/mydecisive/octant/internal/connection/manifest/data"
	manifestmock "github.com/mydecisive/octant/internal/mock/manifest"
	manifestdatamock "github.com/mydecisive/octant/internal/mock/manifestdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestManifestGenerator_All(t *testing.T) {
	t.Parallel()

	var input manifestdata.AllInput
	require.NoError(t, faker.FakeData(&input))
	appData := manifestdata.AppTemplateData{
		Name:      faker.Word(),
		Version:   faker.Word(),
		Namespace: faker.Word(),
	}
	var connData *manifestdata.ConnectionTemplateData
	require.NoError(t, faker.FakeData(&connData))
	var valData manifestdata.ValidatorTemplateData
	require.NoError(t, faker.FakeData(&valData))

	format := manifestdata.YAML
	conn := manifestdata.COLLECTORLB
	template := []byte(faker.Word())
	connMap := map[manifestdata.Connection][]byte{
		conn: template,
	}
	val := manifestdata.TELEMETRY
	valMap := map[manifestdata.Validator][]byte{
		val: template,
	}
	rendered := [][]byte{
		[]byte(faker.Word()),
	}

	mockProvider := manifestmock.NewMockTemplateProvider(t)
	mockProvider.EXPECT().GetApp(manifestdata.MDAI).Return(template, nil).Once()
	mockProvider.EXPECT().GetApp(manifestdata.CERT).Return(template, nil).Once()
	mockProvider.EXPECT().GetApp(manifestdata.CONNECTION).Return(template, nil).Once()
	mockProvider.EXPECT().GetApp(manifestdata.VALIDATOR).Return(template, nil).Once()
	mockProvider.EXPECT().GetAllConnections().Return(connMap, nil).Once()
	mockProvider.EXPECT().GetAllValidators().Return(valMap, nil).Once()

	mockMapper := manifestdatamock.NewMockMapper(t)
	mockMapper.EXPECT().AppTemplateData(manifestdata.MDAI, mock.Anything, mock.Anything, mock.Anything).Return(appData).Once()
	mockMapper.EXPECT().AppTemplateData(manifestdata.CERT, mock.Anything, mock.Anything, mock.Anything).Return(appData).Once()
	mockMapper.EXPECT().AppTemplateData(manifestdata.CONNECTION, mock.Anything, mock.Anything, mock.Anything).Return(appData).Once()
	mockMapper.EXPECT().AppTemplateData(manifestdata.VALIDATOR, mock.Anything, mock.Anything, mock.Anything).Return(appData).Once()
	mockMapper.EXPECT().ConnectionTemplateData(mock.Anything, mock.Anything).Return(connData, nil).Once()
	mockMapper.EXPECT().ValidatorTemplateData(mock.Anything).Return(valData).Once()

	mockRenderer := manifestmock.NewMockTemplateRenderer(t)
	mockRenderer.EXPECT().Render(manifestdata.MDAI.String(), template, format, mock.Anything).Return(rendered, nil).Once()
	mockRenderer.EXPECT().Render(manifestdata.CERT.String(), template, format, mock.Anything).Return(rendered, nil).Once()
	mockRenderer.EXPECT().Render(manifestdata.CONNECTION.String(), template, format, mock.Anything).Return(rendered, nil).Once()
	mockRenderer.EXPECT().Render(manifestdata.VALIDATOR.String(), template, format, mock.Anything).Return(rendered, nil).Once()
	mockRenderer.EXPECT().Render(conn.String(), template, format, connData).Return(rendered, nil).Once()
	mockRenderer.EXPECT().Render(val.String(), template, format, valData).Return(rendered, nil).Once()

	target := NewManifestGenerator(mockProvider, mockRenderer, mockMapper)
	actual, err := target.All(t.Context(), input, format)
	require.NoError(t, err)
	assert.Contains(t, actual, manifestdata.MDAI.String()+"0.yaml")
	assert.Contains(t, actual, manifestdata.CERT.String()+"0.yaml")
	assert.Contains(t, actual, manifestdata.CONNECTION.String()+"0.yaml")
	assert.Contains(t, actual, manifestdata.VALIDATOR.String()+"0.yaml")
	assert.Contains(t, actual, conn.String()+"0.yaml")
	assert.Contains(t, actual, val.String()+"0.yaml")
}

func TestManifestGenerator_App(t *testing.T) {
	t.Parallel()

	app := manifestdata.CERT
	data := manifestdata.AppTemplateData{
		Name:      faker.Word(),
		Version:   faker.Word(),
		Namespace: faker.Word(),
	}
	format := manifestdata.YAML
	template := []byte(faker.Word())

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		expected := [][]byte{
			[]byte(faker.Word()),
		}

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetApp(app).Return(template, nil).Once()

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)
		mockRenderer.EXPECT().Render(app.String(), template, format, data).Return(expected, nil).Once()

		target := NewManifestGenerator(mockProvider, mockRenderer, nil)
		actual, err := target.App(app, data, format)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("err get template", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetApp(app).Return(nil, assert.AnError).Once()

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)

		target := NewManifestGenerator(mockProvider, mockRenderer, nil)
		actual, err := target.App(app, data, format)

		assert.Nil(t, actual)
		assert.ErrorIs(t, err, ErrGetTemplate)
	})

	t.Run("err render template", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetApp(app).Return(template, nil).Once()

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)
		mockRenderer.EXPECT().Render(app.String(), template, format, data).Return(nil, assert.AnError).Once()

		target := NewManifestGenerator(mockProvider, mockRenderer, nil)
		actual, err := target.App(app, data, format)

		assert.Nil(t, actual)
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func TestManifestGenerator_Connections(t *testing.T) {
	t.Parallel()

	var input manifestdata.ConnectionInput
	require.NoError(t, faker.FakeData(&input))

	var data *manifestdata.ConnectionTemplateData
	require.NoError(t, faker.FakeData(&data))

	format := manifestdata.YAML
	conn := manifestdata.COLLECTORLB
	template := []byte(faker.Word())
	connMap := map[manifestdata.Connection][]byte{
		conn: template,
	}
	rendered := [][]byte{
		[]byte(faker.Word()),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetAllConnections().Return(connMap, nil).Once()

		mockMapper := manifestdatamock.NewMockMapper(t)
		mockMapper.EXPECT().ConnectionTemplateData(mock.Anything, input).Return(data, nil).Once()

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)
		mockRenderer.EXPECT().Render(conn.String(), template, format, data).Return(rendered, nil).Once()

		target := NewManifestGenerator(mockProvider, mockRenderer, mockMapper)
		actual, err := target.Connections(t.Context(), input, format)
		require.NoError(t, err)
		assert.Contains(t, actual, conn.String()+"0.yaml")
	})

	t.Run("err get template", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetAllConnections().Return(nil, assert.AnError).Once()

		mockMapper := manifestdatamock.NewMockMapper(t)

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)

		target := NewManifestGenerator(mockProvider, mockRenderer, mockMapper)
		actual, err := target.Connections(t.Context(), input, format)
		assert.Empty(t, actual)
		assert.ErrorIs(t, err, ErrGetTemplate)
	})

	t.Run("err convert to data", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetAllConnections().Return(connMap, nil).Once()

		mockMapper := manifestdatamock.NewMockMapper(t)
		mockMapper.EXPECT().ConnectionTemplateData(mock.Anything, input).Return(data, assert.AnError).Once()

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)

		target := NewManifestGenerator(mockProvider, mockRenderer, mockMapper)
		actual, err := target.Connections(t.Context(), input, format)
		assert.Empty(t, actual)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("err render", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetAllConnections().Return(connMap, nil).Once()

		mockMapper := manifestdatamock.NewMockMapper(t)
		mockMapper.EXPECT().ConnectionTemplateData(mock.Anything, input).Return(data, nil).Once()

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)
		mockRenderer.EXPECT().Render(conn.String(), template, format, data).Return(nil, assert.AnError).Once()

		target := NewManifestGenerator(mockProvider, mockRenderer, mockMapper)
		actual, err := target.Connections(t.Context(), input, format)
		assert.Empty(t, actual)
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func TestManifestGenerator_Validators(t *testing.T) {
	t.Parallel()

	var input manifestdata.ValidatorInput
	require.NoError(t, faker.FakeData(&input))

	var data manifestdata.ValidatorTemplateData
	require.NoError(t, faker.FakeData(&data))

	format := manifestdata.YAML
	val := manifestdata.TELEMETRY
	template := []byte(faker.Word())
	valMap := map[manifestdata.Validator][]byte{
		val: template,
	}
	rendered := [][]byte{
		[]byte(faker.Word()),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetAllValidators().Return(valMap, nil).Once()

		mockMapper := manifestdatamock.NewMockMapper(t)
		mockMapper.EXPECT().ValidatorTemplateData(input).Return(data).Once()

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)
		mockRenderer.EXPECT().Render(val.String(), template, format, data).Return(rendered, nil).Once()

		target := NewManifestGenerator(mockProvider, mockRenderer, mockMapper)
		actual, err := target.Validators(input, format)
		require.NoError(t, err)
		assert.Contains(t, actual, val.String()+"0.yaml")
	})

	t.Run("err get template", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetAllValidators().Return(nil, assert.AnError).Once()

		mockMapper := manifestdatamock.NewMockMapper(t)

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)

		target := NewManifestGenerator(mockProvider, mockRenderer, mockMapper)
		actual, err := target.Validators(input, format)
		assert.Empty(t, actual)
		assert.ErrorIs(t, err, ErrGetTemplate)
	})

	t.Run("err render", func(t *testing.T) {
		t.Parallel()

		mockProvider := manifestmock.NewMockTemplateProvider(t)
		mockProvider.EXPECT().GetAllValidators().Return(valMap, nil).Once()

		mockMapper := manifestdatamock.NewMockMapper(t)
		mockMapper.EXPECT().ValidatorTemplateData(input).Return(data).Once()

		mockRenderer := manifestmock.NewMockTemplateRenderer(t)
		mockRenderer.EXPECT().Render(val.String(), template, format, data).Return(nil, assert.AnError).Once()

		target := NewManifestGenerator(mockProvider, mockRenderer, mockMapper)
		actual, err := target.Validators(input, format)
		assert.Empty(t, actual)
		assert.ErrorIs(t, err, assert.AnError)
	})
}
