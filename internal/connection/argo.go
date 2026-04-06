package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mydecisive/mdai-gateway/internal/integration"
)

type ArgoApp struct {
	Status ArgoAppStatus `json:"status"`
}

type ArgoAppStatus struct {
	Resources []ArgoAppResources `json:"resources"`
	Health    ArgoAppHealth      `json:"health"`
}

type ArgoAppHealth struct {
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}

type ArgoAppResources struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (oc *OctantConnection) getArgoAppStatus(ctx context.Context, name string, namespace string, connection OctantConnectionData) (*ArgoApp, error) {
	argoIntegration, getArgoIntErr := oc.argoClient.GetIntegrationByName(ctx, namespace, connection.Deployment.IntegrationName)
	if getArgoIntErr != nil {
		return nil, getArgoIntErr
	}

	// GET APP
	getAppURL := fmt.Sprintf("%s/api/v1/applications/%s?upsert=true", argoIntegration.APIUrl, name)
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
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var app ArgoApp
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

	if appCreateErr := oc.doArgoAppCreation(ctx, templateData, argoIntegration); appCreateErr != nil {
		return appCreateErr
	}

	return oc.doArgoAppSync(ctx, templateData, argoIntegration, name)
}

func (oc *OctantConnection) doArgoAppSync(ctx context.Context, templateData *ArgoTemplateData, argoIntegration *integration.ArgoCDIntegrationData, name string) error {
	manifests, err := renderCollectorDeploymentManifests(templateData, JSONOutputFormat)
	if err != nil {
		return err
	}

	var manifestsSlice []string
	for _, manifest := range manifests {
		manifestsSlice = append(manifestsSlice, string(manifest))
	}

	// TODO: Make a struct for this
	syncPayload := map[string]any{
		"revision": "HEAD",
		"prune":    false,
		"dryRun":   false,
		"strategy": map[string]any{
			"apply": map[string]bool{
				"force": false,
			},
		},
		"manifests": manifestsSlice,
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
		// TODO: Handle this error better. Return the argo response too, and/or pass along better messages
		return fmt.Errorf("unexpected status code: %d", syncResp.StatusCode)
	}
	return nil
}

func (oc *OctantConnection) doArgoAppCreation(ctx context.Context, templateData *ArgoTemplateData, argoIntegration *integration.ArgoCDIntegrationData) error {
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
		// TODO: Handle this error better. Return the argo response too, and/or pass along better messages
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
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
		// TODO: Handle this error better. Return the argo response too, and/or pass along better messages
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
