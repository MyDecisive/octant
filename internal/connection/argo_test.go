package connection

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mydecisive/octant/internal/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDeleteArgoApp(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/api/v1/applications/my-app", r.URL.Path)
		assert.Contains(t, r.URL.RawQuery, "cascade=true")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)

	octantConnection := f.build()

	err := octantConnection.deleteArgoApp(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})
	require.NoError(t, err)
}

func TestPushArgoApp(t *testing.T) { // nolint:gocognit
	t.Parallel()

	tests := []struct {
		name               string
		destinations       []OctantConnectionDestination
		expectDatadogCall  bool
		ddClientErr        error
		expectArgoCall     bool
		argoClientErr      error
		createResponseCode int
		syncResponseCode   int
		expectedErr        string
	}{
		// (Same test cases...)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost && r.URL.Path == "/api/v1/applications" {
					w.WriteHeader(tc.createResponseCode)
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == "/api/v1/applications/my-test-app/sync" {
					w.WriteHeader(tc.syncResponseCode)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			f := setupFixture(t)
			f.httpClient = ts.Client()

			if tc.expectArgoCall {
				if tc.argoClientErr != nil {
					f.argoMock.EXPECT().
						GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
						Return(nil, tc.argoClientErr)
				} else {
					f.argoMock.EXPECT().
						GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
						Return(&integration.ArgoCDIntegrationData{
							APIUrl:       ts.URL,
							AccountToken: "fake-token",
						}, nil)
				}
			}

			if tc.expectDatadogCall {
				if tc.ddClientErr != nil {
					f.datadogMock.EXPECT().
						GetIntegrationByName(mock.Anything, defaultNamespace, "dd-1").
						Return(nil, tc.ddClientErr)
				} else {
					f.datadogMock.EXPECT().
						GetIntegrationByName(mock.Anything, defaultNamespace, "dd-1").
						Return(&integration.DataDogIntegrationData{}, nil)
				}
			}

			octantConnection := f.build()

			connData := OctantConnectionData{
				Destinations: tc.destinations,
				Deployment: &Deployment{
					IntegrationName: "argo-test",
				},
			}

			err := octantConnection.sideloadConnectionApp(context.Background(), "default", "my-test-app", connData)

			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeleteArgoApp_Error_IntegrationFetchFailed(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
		Return(nil, errors.New("injected argo integration error"))

	octantConnection := f.build()

	err := octantConnection.deleteArgoApp(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected argo integration error")
}

func TestDeleteArgoApp_Error_RequestCreation(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: "://invalid-url",
		}, nil)

	octantConnection := f.build()

	err := octantConnection.deleteArgoApp(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
}

func TestDeleteArgoApp_Error_HTTPDoFailed(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)

	octantConnection := f.build()

	err := octantConnection.deleteArgoApp(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
}

func TestDeleteArgoApp_Error_BadStatusCode_Unauthorized_Error(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte(`{"error": "invalid session: token signature is invalid: signature is invalid",
		"code": 16,
		"message": "invalid session: token signature is invalid: signature is invalid"}`))
		assert.NoError(t, err)
	}))
	defer ts.Close()

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)

	octantConnection := f.build()

	err := octantConnection.deleteArgoApp(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "'argo-test' token invalid:")
	assert.Contains(t, err.Error(), "\n{\n  \"error\":")
}

func TestDeleteArgoApp_Error_BadStatusCode_String_Error(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`ooky spooky`)) // nolint: errcheck,gosec,revive
	}))
	defer ts.Close()

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)

	octantConnection := f.build()

	err := octantConnection.deleteArgoApp(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "got unexpected response code from ArgoCD API")
	assert.Contains(t, err.Error(), "Status 500")
	assert.Contains(t, err.Error(), "Body: ooky spooky")
}

func TestPushArgoApp_Error_HTTPDoFailed(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").
		Return(&integration.ArgoCDIntegrationData{
			APIUrl: ts.URL,
		}, nil)
	f.datadogMock.EXPECT().
		GetIntegrationByName(mock.Anything, defaultNamespace, "dd-1").
		Return(&integration.DataDogIntegrationData{}, nil)

	octantConnection := f.build()

	connData := OctantConnectionData{
		Destinations: []OctantConnectionDestination{
			{DestinationType: "datadog", IntegrationName: "dd-1"},
		},
		Deployment: &Deployment{IntegrationName: "argo-test"},
	}

	err := octantConnection.sideloadConnectionApp(context.Background(), "default", "my-test-app", connData)
	require.Error(t, err)
}
