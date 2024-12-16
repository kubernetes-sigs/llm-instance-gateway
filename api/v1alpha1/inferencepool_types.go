/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InferencePoolSpec defines the desired state of InferencePool
type InferencePoolSpec struct {

	// Selector uses a map of label to watch model server pods
	// that should be included in the InferencePool. ModelServers should not
	// be with any other Service or InferencePool, that behavior is not supported
	// and will result in sub-optimal utilization.
	// In some cases, implementations may translate this to a Service selector, so this matches the simple
	// map used for Service selectors instead of the full Kubernetes LabelSelector type.
	//
	// +kubebuilder:validation:Required
	Selector map[LabelString]LabelString `json:"selector,omitempty"`

	// TargetPort is the port number that the model servers within the pool expect
	// to recieve traffic from.
	// This maps to the TargetPort in: https://pkg.go.dev/k8s.io/api/core/v1#ServicePort
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Required
	TargetPort int32 `json:"targetPort,omitempty"`
}

// InferencePoolStatus defines the observed state of InferencePool
type InferencePoolStatus struct {

	// Conditions track the state of the InferencePool.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// This indirection allows us to validate this type, since there is not native map validation support
//
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=63
type LabelString string

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// InferencePool is the Schema for the Inferencepools API
type InferencePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferencePoolSpec   `json:"spec,omitempty"`
	Status InferencePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InferencePoolList contains a list of InferencePool
type InferencePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferencePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferencePool{}, &InferencePoolList{})
}
