package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mydecisive/octant/internal/integration"
)

type argoApp struct {
	Status argoAppStatus `json:"status"`
}

type argoAppStatus struct {
	Resources []argoAppResources `json:"resources"`
	Health    argoAppHealth      `json:"health"`
}

type argoAppHealth struct {
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}

type argoAppResources struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type argoSyncPayload struct {
	Revision  string           `json:"revision"`
	Prune     bool             `json:"prune"`
	DryRun    bool             `json:"dryRun"`
	Strategy  argoSyncStrategy `json:"strategy"`
	Manifests []string         `json:"manifests"`
}

type argoSyncStrategy struct {
	Apply argoSyncApply `json:"apply"`
}

type argoSyncApply struct {
	Force bool `json:"force"`
}

func (oc *OctantConnection) getArgoAppStatus(ctx context.Context, name string, namespace string, connection OctantConnectionData) (*argoApp, error) {
	argoIntegration, getArgoIntErr := oc.argoClient.GetIntegrationByName(ctx, namespace, connection.Deployment.IntegrationName)
	if getArgoIntErr != nil {
		return nil, getArgoIntErr
	}

	getAppURL := fmt.Sprintf("%s/api/v1/applications/%s", argoIntegration.APIUrl, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getAppURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+argoIntegration.AccountToken)
	resp, err := oc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, handleArgoErrorResponse(resp, connection)
	}
	var app argoApp
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, err
	}

	return &app, nil
}

func (oc *OctantConnection) pushArgoApp(ctx context.Context, namespace, name string, connection OctantConnectionData) error {
	templateData, err := oc.createTemplateData(ctx, namespace, name, connection)
	if err != nil {
		return err
	}

	argoIntegration, getArgoIntErr := oc.argoClient.GetIntegrationByName(ctx, namespace, connection.Deployment.IntegrationName)
	if getArgoIntErr != nil {
		return getArgoIntErr
	}
	if argoIntegration == nil {
		return fmt.Errorf("no ArgoCD integration found with name %s", connection.Deployment.IntegrationName)
	}

	if appCreateErr := oc.doArgoAppCreation(ctx, templateData, connection, argoIntegration); appCreateErr != nil {
		return appCreateErr
	}

	return oc.doArgoAppSync(ctx, templateData, connection, argoIntegration, name)
}

func (oc *OctantConnection) doArgoAppSync(ctx context.Context, templateData *ArgoTemplateData, connection OctantConnectionData, argoIntegration *integration.ArgoCDIntegrationData, name string) error {
	manifests, err := renderCollectorDeploymentManifests(templateData, JSONOutputFormat)
	if err != nil {
		return err
	}

	var manifestsSlice []string
	for _, manifest := range manifests {
		manifestsSlice = append(manifestsSlice, string(manifest))
	}

	syncPayload := argoSyncPayload{
		Revision: "HEAD",
		Prune:    false,
		DryRun:   false,
		Strategy: argoSyncStrategy{
			Apply: argoSyncApply{
				Force: false,
			},
		},
		Manifests: manifestsSlice,
	}

	syncPayloadJSON, err := json.Marshal(syncPayload)
	if err != nil {
		return err
	}
	syncURL := fmt.Sprintf("%s/api/v1/applications/%s/sync", argoIntegration.APIUrl, name)
	syncReq, err := http.NewRequestWithContext(ctx, http.MethodPost, syncURL, bytes.NewReader(syncPayloadJSON))
	if err != nil {
		return err
	}
	syncReq.Header.Set("Content-Type", "application/json")
	syncReq.Header.Set("Authorization", "Bearer "+argoIntegration.AccountToken)
	syncResp, err := oc.httpClient.Do(syncReq)
	if err != nil {
		// TODO: Handle this error better
		return err
	}
	defer func() {
		_ = syncResp.Body.Close()
	}()
	if syncResp.StatusCode != http.StatusOK {
		return handleArgoErrorResponse(syncResp, connection)
	}
	return nil
}

func (oc *OctantConnection) doArgoAppCreation(ctx context.Context, templateData *ArgoTemplateData, connection OctantConnectionData, argoIntegration *integration.ArgoCDIntegrationData) error {
	appJSON, err := renderArgoAppManifest(templateData, JSONOutputFormat)
	if err != nil {
		return err
	}
	createAppURL := argoIntegration.APIUrl + "/api/v1/applications"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createAppURL, bytes.NewReader(appJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+argoIntegration.AccountToken)
	resp, err := oc.httpClient.Do(req)
	if err != nil {
		// TODO: Handle this error better
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return handleArgoErrorResponse(resp, connection)
	}
	return nil
}

func (oc *OctantConnection) deleteArgoApp(ctx context.Context, name string, namespace string, connection OctantConnectionData) error {
	argoIntegration, getArgoIntErr := oc.argoClient.GetIntegrationByName(ctx, namespace, connection.Deployment.IntegrationName)
	if getArgoIntErr != nil {
		return getArgoIntErr
	}

	query := "?cascade=true&propagationPolicy=foreground&appNamespace=argocd&cascade=true"
	deleteAppURL := fmt.Sprintf("%s/api/v1/applications/%s%s", argoIntegration.APIUrl, name, query)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteAppURL, http.NoBody)
	if err != nil {
		return err
	}
	// Despite no body being required, ArgoCD requires a JSON content type to process Delete
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+argoIntegration.AccountToken)
	resp, err := oc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return handleArgoErrorResponse(resp, connection)
	}
	return nil
}

func handleArgoErrorResponse(resp *http.Response, connection OctantConnectionData) error {
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("unexpected status %d; also failed to read error body: %w", resp.StatusCode, readErr)
	}

	var bodyStr string
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err == nil {
		bodyStr = "\n" + prettyJSON.String()
	} else {
		bodyStr = string(bytes.TrimSpace(bodyBytes))
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf(
			"got 401 forbidden response from ArgoCD API. Account token in ArgoCD integration '%s' may be incorrect or expired. Response body: %v",
			connection.Deployment.IntegrationName,
			bodyStr,
		)
	default:
		return fmt.Errorf("got unexpected response code from ArgoCD API: Status %d, Body: %s", resp.StatusCode, bodyStr)
	}
}
