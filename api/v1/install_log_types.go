// +kubebuilder:object:generate=true
// +groupName=octant.mydecisive.ai

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:rbac:groups=octant.mydecisive.ai,resources=octantinstalllogs,verbs=get;list;watch;create;update;patch;delete

func GetOctantInstallLogGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "octant.mydecisive.ai",
		Version:  "v1",
		Resource: "octantinstalllogs",
	}
}

// OctantInstallLog is the Schema for the setup logs API
type OctantInstallLog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OctantInstallLogSpec `json:"spec,omitempty"`
}

// OctantInstallLogSpec defines the data schema
type OctantInstallLogSpec struct {
	// +kubebuilder:validation:Optional
	Events []OctantInstallEvent `json:"events"`
}

type OctantInstallEventResultCode string

const OctantInstallEventResultSuccess OctantInstallEventResultCode = "SUCCESS"
const OctantInstallEventResultPartialSuccess OctantInstallEventResultCode = "PARTIAL_SUCCESS"
const OctantInstallEventResultError OctantInstallEventResultCode = "ERROR"

type OctantInstallEvent struct {
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
	// +kubebuilder:validation:Optional
	Result OctantInstallEventResultCode `json:"result"`
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:Optional
	Ref string `json:"ref"`
	// +kubebuilder:validation:Optional
	Type string `json:"type"`
	// +kubebuilder:validation:Optional
	Message string `json:"message"`
}

// +kubebuilder:object:root=true

// OctantInstallLogList contains a list of OctantInstallLogs
type OctantInstallLogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OctantInstallLog `json:"items"`
}
