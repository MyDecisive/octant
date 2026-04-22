package connection

type DeploymentType string

type Deployment struct {
	Type            DeploymentType `json:"type"`
	IntegrationName string         `json:"integrationName"`
}

// TODO: Refactor connection operations to use tasksets/plans instead of if-argo-then
// type DeploymentTask func(ctx context.Context, name string, namespace string, connection OctantConnectionData) (any, error)
// type DeploymentTaskSet map[string][]DeploymentTask.

const (
	ArgoSideloadDeploymentType  DeploymentType = "argocd-sideload"
	ArgoManifestsDeploymentType DeploymentType = "argocd-manifests"
)
