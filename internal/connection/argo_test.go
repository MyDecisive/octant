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

func TestGetArgoAppStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverResponse int
		responseBody   string
		expectError    bool
	}{
		{
			name:           "success",
			serverResponse: http.StatusOK,
			responseBody:   `{"status": {"health": {"status": "Healthy"}}}`,
			expectError:    false,
		},
		{
			name:           "argo error",
			serverResponse: http.StatusNotFound,
			responseBody:   `{"error": "not found"}`,
			expectError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/applications/my-app", r.URL.Path)
				assert.Equal(t, "Bearer fake-token", r.Header.Get("Authorization"))

				w.WriteHeader(tc.serverResponse)
				w.Write([]byte(tc.responseBody)) // nolint: errcheck,gosec
			}))
			defer ts.Close()

			f := setupFixture(t)
			f.httpClient = ts.Client()
			f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
				APIUrl:       ts.URL,
				AccountToken: "fake-token",
			}, nil)

			octantConnection := f.build()

			app, err := octantConnection.getArgoAppStatus(context.Background(), "my-app", "default", OctantConnectionData{
				Deployment: &Deployment{IntegrationName: "argo-test"},
			})

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "Healthy", app.Status.Health.Status)
			}
		})
	}
}

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
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
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
					f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(nil, tc.argoClientErr)
				} else {
					f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
						APIUrl:       ts.URL,
						AccountToken: "fake-token",
					}, nil)
				}
			}

			if tc.expectDatadogCall {
				if tc.ddClientErr != nil {
					f.datadogMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "dd-1").Return(nil, tc.ddClientErr)
				} else {
					f.datadogMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "dd-1").Return(&integration.DataDogIntegrationData{}, nil)
				}
			}

			octantConnection := f.build()

			connData := OctantConnectionData{
				Destinations: tc.destinations,
				Deployment: &Deployment{
					IntegrationName: "argo-test",
				},
			}

			err := octantConnection.pushArgoApp(context.Background(), "default", "my-test-app", connData)

			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetArgoAppStatus_Error_IntegrationFetchFailed(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(nil, errors.New("injected argo integration error"))

	octantConnection := f.build()

	_, err := octantConnection.getArgoAppStatus(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected argo integration error")
}

func TestGetArgoAppStatus_Error_RequestCreation(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
		APIUrl: "://invalid-url",
	}, nil)

	octantConnection := f.build()

	_, err := octantConnection.getArgoAppStatus(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
}

func TestGetArgoAppStatus_Error_HTTPDoFailed(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close() // closed immediately to force connection failure

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
		APIUrl: ts.URL,
	}, nil)

	octantConnection := f.build()

	_, err := octantConnection.getArgoAppStatus(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
}

func TestGetArgoAppStatus_Error_InvalidJSON(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{ "invalid": json `)) // nolint: errcheck,gosec,revive
	}))
	defer ts.Close()

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
		APIUrl: ts.URL,
	}, nil)

	octantConnection := f.build()

	_, err := octantConnection.getArgoAppStatus(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
}

func TestDeleteArgoApp_Error_IntegrationFetchFailed(t *testing.T) {
	t.Parallel()

	f := setupFixture(t)
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(nil, errors.New("injected argo integration error"))

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
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
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
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
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
		w.Write([]byte(`{"error": "invalid session: token signature is invalid: signature is invalid","code": 16,"message": "invalid session: token signature is invalid: signature is invalid"}`)) // nolint: errcheck,gosec,revive
	}))
	defer ts.Close()

	f := setupFixture(t)
	f.httpClient = ts.Client()
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
		APIUrl: ts.URL,
	}, nil)

	octantConnection := f.build()

	err := octantConnection.deleteArgoApp(context.Background(), "my-app", "default", OctantConnectionData{
		Deployment: &Deployment{IntegrationName: "argo-test"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "got 401 forbidden response from ArgoCD API")
	assert.Contains(t, err.Error(), "Account token in ArgoCD integration 'argo-test' may be incorrect or expired.")
	assert.Contains(t, err.Error(), "Response body: \n{\n  \"error\":")
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
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
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
	f.argoMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "argo-test").Return(&integration.ArgoCDIntegrationData{
		APIUrl: ts.URL,
	}, nil)
	f.datadogMock.EXPECT().GetIntegrationByName(mock.Anything, defaultNamespace, "dd-1").Return(&integration.DataDogIntegrationData{}, nil)

	octantConnection := f.build()

	connData := OctantConnectionData{
		Destinations: []OctantConnectionDestination{
			{DestinationType: "datadog", IntegrationName: "dd-1"},
		},
		Deployment: &Deployment{IntegrationName: "argo-test"},
	}

	err := octantConnection.pushArgoApp(context.Background(), "default", "my-test-app", connData)
	require.Error(t, err)
}
