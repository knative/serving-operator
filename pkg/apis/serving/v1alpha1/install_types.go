package v1alpha1

import (
	"github.com/knative/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	InstallSucceeded            apis.ConditionType = "InstallSucceeded"
	InstallDeploymentsAvailable apis.ConditionType = "DeploymentsAvailable"
)

// InstallSpec defines the desired state of Install
// +k8s:openapi-gen=true
type InstallSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// A means to override the corresponding entries in the upstream configmaps
	// +optional
	Config map[string]map[string]string `json:"config,omitempty"`
}

// InstallStatus defines the observed state of Install
// +k8s:openapi-gen=true
type InstallStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags:
	// https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// The version of the installed release
	Version string `json:"version,omitempty"`
	// The latest available observations of a resource's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions apis.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Install is the Schema for the installs API
// +k8s:openapi-gen=true
type Install struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstallSpec   `json:"spec,omitempty"`
	Status InstallStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InstallList contains a list of Install
type InstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Install `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Install{}, &InstallList{})
}
