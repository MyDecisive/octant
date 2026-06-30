// +kubebuilder:object:generate=true
// +groupName=octant.mydecisive.ai

package v1

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:rbac:groups=octant.mydecisive.ai,resources=octantinstalllogs,verbs=get;list;watch;create;update;patch;delete

const (
	octantInstallLogGroup             = "octant.mydecisive.ai"
	octantInstallLogVersion           = "v1"
	octantInstallLogPlural            = "octantinstalllogs"
	octantInstallLogKind              = "OctantInstallLog"
	octantInstallEventTimestampFormat = "2006-01-02_15-04-05.999999"
)

func CreateOctantIntallEventTimestamp() string {
	return time.Now().UTC().Format(octantInstallEventTimestampFormat)
}

func GetOctantInstallEventTimestamp(timestamp string) (time.Time, error) {
	return time.Parse(octantInstallEventTimestampFormat, timestamp)
}

func GetOctantInstallLogKind() string {
	return octantInstallLogKind
}

func GetOctantInstallLogAPIVersion() string {
	return fmt.Sprintf("%s/%s", octantInstallLogGroup, octantInstallLogVersion)
}

func GetOctantInstallLogGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    octantInstallLogGroup,
		Version:  octantInstallLogVersion,
		Resource: octantInstallLogPlural,
	}
}

// OctantInstallLog is the Schema for the setup logs API.
type OctantInstallLog struct {
	metav1.TypeMeta   `json:",inline"` // nolint:revive
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OctantInstallLogSpec `json:"spec,omitempty"`
}

// OctantInstallLogSpec defines the data schema.
type OctantInstallLogSpec struct {
	// +kubebuilder:validation:Optional
	Events []OctantInstallEvent `json:"events"`
}

type OctantInstallEventResultCode string

const (
	// SuccessOctantInstallEventResult indicates that this event was completely successful and the system is ready to
	// progress to the next step of the install flow.
	SuccessOctantInstallEventResult OctantInstallEventResultCode = "SUCCESS"
	// FailureOctantInstallEventResult indicates a complete failure of the attempted action, meaning that no state
	// change was achieved, and a retry of the same action is viable.
	FailureOctantInstallEventResult OctantInstallEventResultCode = "FAILURE"
	// PartialSuccessOctantInstallEventResult indicates that this event was only partially successful, tainting the
	// destination system, indicating that the system is in a state that likely requires manual intervention to proceed
	// with Octant's flow.
	PartialSuccessOctantInstallEventResult OctantInstallEventResultCode = "PARTIAL_SUCCESS"
)

type OctantInstallEventAction string

const (
	// CreateDeployIntegrationOctantInstallEventAction is the action of creating an integration within Octant that
	// allows for modifying the destination cluster.
	CreateDeployIntegrationOctantInstallEventAction OctantInstallEventAction = "CREATE_DEPLOY_INTEGRATION"
	// InstallMDAIHubOctantInstallEventAction is the action of installing the MDAI hub components.
	InstallMDAIHubOctantInstallEventAction OctantInstallEventAction = "INSTALL_MDAI_HUB"
	// CreateDestinationIntegrationOctantInstallEventAction is the action of creating a telemetry destination
	// integration that connections can consume.
	CreateDestinationIntegrationOctantInstallEventAction OctantInstallEventAction = "CREATE_DESTINATION_INTEGRATION"
	// CreateConnectionOctantInstallEventAction is the action of creating a connection and all underlying
	// infrastructure.
	CreateConnectionOctantInstallEventAction OctantInstallEventAction = "CREATE_CONNECTION"
	// NNFClientsConnectedVerifiedOctantInstallEventAction is the action of completing the envoy connected clients
	// validation loop.
	NNFClientsConnectedVerifiedOctantInstallEventAction OctantInstallEventAction = "NNF_CLIENTS_CONNECTED_VERIFIED"
	// IngressVerifiedOctantInstallEventAction is the action of completing the ingress validation loop (data has been
	// received).
	IngressVerifiedOctantInstallEventAction OctantInstallEventAction = "INGRESS_VERIFIED"
	// EgressVerifiedOctantInstallEventAction is the action of completing the egress validation loop (data has been
	// sent).
	EgressVerifiedOctantInstallEventAction OctantInstallEventAction = "EGRESS_VERIFIED"
	// ValidationPassedOctantInstallEventAction is the action of completing the policy or parity data validation loop
	// (data is satisfactorily the same in/out).
	ValidationPassedOctantInstallEventAction OctantInstallEventAction = "VALIDATION_PASSED"
)

type OctantInstallLogEventActionDeployIntegrationSubtype string

const (
	ArgoCDOctantInstallLogEventActionDeployIntegrationSubtype OctantInstallLogEventActionDeployIntegrationSubtype = "ARGOCD" // nolint:lll
)

type OctantInstallEvent struct {
	// Action is the type of step in progressing the Octant/MDAI system towards the installed state
	Action OctantInstallEventAction `json:"action"`
	// Timestamp is the timestamp of when this state change was observed
	Timestamp string `json:"timestamp"`
	// Result is the success/partial-success/failure state of this event
	// +kubebuilder:validation:Optional
	Result OctantInstallEventResultCode `json:"result"`
	// Namespace is the namespace this event took place in
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace"`
	// Ref is the integration/connection/other resource that this action targeted
	// +kubebuilder:validation:Optional
	Ref string `json:"ref"`
	// Subtype is the specific underlying type of the integration/other (Argo for deploy integration, Datadog for
	// destination)
	// +kubebuilder:validation:Optional
	Subtype string `json:"subtype"`
	// Message is an optional message to help further diagnosis of failure/partial-success events
	// +kubebuilder:validation:Optional
	Message string `json:"message"`
}

// +kubebuilder:object:root=true

// OctantInstallLogList contains a list of OctantInstallLogs.
type OctantInstallLogList struct {
	metav1.TypeMeta `json:",inline"` // nolint:revive
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []OctantInstallLog `json:"items"`
}
