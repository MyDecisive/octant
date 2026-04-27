package connection

type DeploymentType string

type Deployment struct {
	Type            DeploymentType `json:"type"`
	IntegrationName string         `json:"integrationName"`
}

const (
	ArgoSideloadDeploymentType  DeploymentType = "argocd-sideload"
	ArgoManifestsDeploymentType DeploymentType = "argocd-manifests"
)
