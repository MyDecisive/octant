package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mydecisive/octant/internal/integration"
)

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

func (oc *OctantConnection) sideloadConnectionApp(
	ctx context.Context,
	namespace, name string,
	connection OctantConnectionData,
) error {
	// TODO: Port all this functionality over to the argocd.Client!

	templateData, err := oc.createTemplateData(ctx, namespace, name, connection)
	if err != nil {
		return err
	}

	argoIntegration, err := oc.getArgoIntegration(ctx, connection)
	if err != nil {
		return err
	}

	if appCreateErr := oc.doArgoAppCreation(ctx, templateData, connection, argoIntegration); appCreateErr != nil {
		return appCreateErr
	}

	return oc.doArgoAppSync(ctx, templateData, connection, argoIntegration, name)
}

func (oc *OctantConnection) getArgoIntegration(
	ctx context.Context,
	connection OctantConnectionData,
) (*integration.ArgoCDIntegrationData, error) {
	argoIntegration, getArgoIntErr := oc.argoClient.GetIntegrationByName(
		ctx,
		connection.Deployment.IntegrationName,
	)
	if getArgoIntErr != nil {
		return nil, getArgoIntErr
	}
	if argoIntegration == nil {
		return nil, fmt.Errorf("no ArgoCD integration found with name %s", connection.Deployment.IntegrationName)
	}
	return argoIntegration, nil
}

func (oc *OctantConnection) doArgoAppSync(
	ctx context.Context,
	templateData *ArgoConnectionTemplateData,
	connection OctantConnectionData,
	argoIntegration *integration.ArgoCDIntegrationData,
	name string,
) error {
	// TODO: Port all this functionality over to the argocd.Client!

	manifests, err := renderCollectorDeploymentManifests(templateData, JSONOutputFormat)
	if err != nil {
		return err
	}

	var manifestsSlice []string
	for _, manifest := range manifests {
		manifestsSlice = append(manifestsSlice, string(manifest))
	}

	syncPayload := makeArgoSyncPayload(manifestsSlice)

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

func (oc *OctantConnection) sideloadValidatorForConnection(
	ctx context.Context,
	connection OctantConnectionData,
	connectionName string,
	namespace string,
) (string, error) {
	// TODO: Port all this functionality over to the argocd.Client!

	// TODO: This is wonky today; we are pushing a new sync on the same app as the connection, but with just the
	//       validator. This should work because prune = false (argo won't remove those other "orphaned" resources). The
	//       entire sideload behavior has this ephemerality problem... but feels weird to push new manifests on top of
	//       the old ones like this. Clean this up for the git integration.

	argoIntegration, err := oc.getArgoIntegration(ctx, connection)
	if err != nil {
		return "", err
	}

	templateData := &ArgoValidatorTemplateData{
		ConnectionName: connectionName,
		Namespace:      namespace,
		ValidatorRunID: getRunID(),
	}

	manifest, err := renderValidatorManifestForConnection(templateData, JSONOutputFormat)
	if err != nil {
		return "", err
	}

	manifestsSlice := []string{
		string(manifest),
	}

	syncPayload := makeArgoSyncPayload(manifestsSlice)

	syncPayloadJSON, err := json.Marshal(syncPayload)
	if err != nil {
		return "", err
	}
	syncURL := fmt.Sprintf("%s/api/v1/applications/%s/sync", argoIntegration.APIUrl, connectionName)
	syncReq, err := http.NewRequestWithContext(ctx, http.MethodPost, syncURL, bytes.NewReader(syncPayloadJSON))
	if err != nil {
		return "", err
	}
	syncReq.Header.Set("Content-Type", "application/json")
	syncReq.Header.Set("Authorization", "Bearer "+argoIntegration.AccountToken)
	syncResp, err := oc.httpClient.Do(syncReq)
	if err != nil {
		// TODO: Handle this error better
		return "", err
	}
	defer func() {
		_ = syncResp.Body.Close()
	}()
	if syncResp.StatusCode != http.StatusOK {
		return "", handleArgoErrorResponse(syncResp, connection)
	}
	return templateData.ValidatorRunID, nil
}

func makeArgoSyncPayload(manifestsSlice []string) argoSyncPayload {
	return argoSyncPayload{
		Revision: "HEAD",
		Prune:    false,
		DryRun:   false,
		Strategy: argoSyncStrategy{
			Apply: argoSyncApply{
				Force: true,
			},
		},
		Manifests: manifestsSlice,
	}
}

func (oc *OctantConnection) doArgoAppCreation(
	ctx context.Context,
	templateData *ArgoConnectionTemplateData,
	connection OctantConnectionData,
	argoIntegration *integration.ArgoCDIntegrationData,
) error {
	// TODO: If possible, use the functionality from argocd.Client

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

func (oc *OctantConnection) deleteArgoApp(ctx context.Context, name string, connection OctantConnectionData) error {
	// TODO: Port functionality over to argocd.Client!

	argoIntegration, getArgoIntErr := oc.argoClient.GetIntegrationByName(
		ctx,
		connection.Deployment.IntegrationName,
	)
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

func (oc *OctantConnection) deleteValidatorResource(
	ctx context.Context,
	name string,
	namespace string,
	connection OctantConnectionData,
) error {
	// TODO: Port functionality over to argocd.Client!

	argoIntegration, getArgoIntErr := oc.argoClient.GetIntegrationByName(
		ctx,
		connection.Deployment.IntegrationName,
	)
	if getArgoIntErr != nil {
		return getArgoIntErr
	}

	// TODO: This is gnarly; we're reaching in and deleting a specific resource on the app. CLEAN THIS UP.
	query := fmt.Sprintf(
		"?namespace=%s&resourceName=%s-telemetry-validation&group=hub.mydecisive.ai&version=v1&kind=TelemetryValidation",
		namespace, name,
	)
	deleteAppURL := fmt.Sprintf("%s/api/v1/applications/%s/resource%s", argoIntegration.APIUrl, name, query)
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
			"'%s' token invalid: %v",
			connection.Deployment.IntegrationName,
			bodyStr,
		)
	default:
		return fmt.Errorf(
			"got unexpected response code from ArgoCD API: Status %d, Body: %s",
			resp.StatusCode,
			bodyStr,
		)
	}
}
