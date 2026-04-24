package budgetfilter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/mydecisive/octant/internal/config"
	wrappermock "github.com/mydecisive/octant/internal/mock/wrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMDAIGateway_GetVariable(t *testing.T) {
	t.Parallel()

	varName := faker.Word()
	varData := faker.Word()
	namespace := faker.Word()
	hubName := faker.Word()

	c := &config.Configuration{
		Budget: config.Budget{
			DefaultMDAIGatewayName: faker.Word(),
		},
	}

	url := fmt.Sprintf(mdaiGatewayRootURLFormatter, c.Budget.DefaultMDAIGatewayName, namespace) + fmt.Sprintf(mdaiGatewayGetVarFormatter, hubName, varName)
	jsonBody, err := json.Marshal(map[string]string{
		varName: varData,
	})
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockClient := wrappermock.NewMockHTTPClient(t)
		mockClient.EXPECT().Get(url).Return(&http.Response{
			Body: io.NopCloser(bytes.NewBuffer(jsonBody)),
		}, nil)

		target := NewMDAIGateway(c, mockClient)

		actual, err := target.GetVariable(namespace, hubName, varName)
		require.NoError(t, err)

		assert.Equal(t, varData, actual)
	})

	t.Run("err get", func(t *testing.T) {
		t.Parallel()

		mockClient := wrappermock.NewMockHTTPClient(t)
		mockClient.EXPECT().Get(url).Return(&http.Response{}, assert.AnError)

		target := NewMDAIGateway(c, mockClient)

		actual, err := target.GetVariable(namespace, hubName, varName)
		assert.Error(t, err)

		assert.Empty(t, actual)
	})

	t.Run("err invalid body", func(t *testing.T) {
		t.Parallel()

		mockClient := wrappermock.NewMockHTTPClient(t)
		mockClient.EXPECT().Get(url).Return(&http.Response{
			Body: io.NopCloser(strings.NewReader(faker.Word())),
		}, nil)

		target := NewMDAIGateway(c, mockClient)

		actual, err := target.GetVariable(namespace, hubName, varName)
		assert.Error(t, err)

		assert.Empty(t, actual)
	})
}

func TestMDAIGateway_UpdateVariable(t *testing.T) {
	t.Parallel()

	varName := faker.Word()
	namespace := faker.Word()
	hubName := faker.Word()

	c := &config.Configuration{
		Budget: config.Budget{
			DefaultMDAIGatewayName: faker.Word(),
		},
	}

	url := fmt.Sprintf(mdaiGatewayRootURLFormatter, c.Budget.DefaultMDAIGatewayName, namespace) + fmt.Sprintf(mdaiGatewayPostVarFormatter, hubName, varName)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mockClient := wrappermock.NewMockHTTPClient(t)
		mockClient.EXPECT().Post(url, contentTypeJSON, mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
		}, nil)

		target := NewMDAIGateway(c, mockClient)

		err := target.UpdateVariable(namespace, hubName, varName, faker.Word())
		assert.NoError(t, err)
	})

	t.Run("err post", func(t *testing.T) {
		t.Parallel()

		mockClient := wrappermock.NewMockHTTPClient(t)
		mockClient.EXPECT().Post(url, contentTypeJSON, mock.Anything).Return(nil, assert.AnError)

		target := NewMDAIGateway(c, mockClient)

		err := target.UpdateVariable(namespace, hubName, varName, faker.Word())
		assert.Error(t, err)
	})

	t.Run("err status", func(t *testing.T) {
		t.Parallel()

		mockClient := wrappermock.NewMockHTTPClient(t)
		mockClient.EXPECT().Post(url, contentTypeJSON, mock.Anything).Return(&http.Response{
			StatusCode: http.StatusInternalServerError,
		}, nil)

		target := NewMDAIGateway(c, mockClient)

		err := target.UpdateVariable(namespace, hubName, varName, faker.Word())
		assert.ErrorIs(t, err, ErrInvalid)
	})
}
