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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LLMServerPoolSpec defines the desired state of LLMServerPool
type LLMServerPoolSpec struct {

	// ServiceRefs select the distinct services to include in the backend pool.
	// NOTE: These services should be consumed by only the llmServerPool they
	// are referenced by. Should this behavior be breached, Instance Gateway
	// behavior is not guaranteed.
	ServiceRefs []corev1.ObjectReference `json:"serviceRefs,omitempty"`
}

// LLMServerPoolStatus defines the observed state of LLMServerPool
type LLMServerPoolStatus struct {

	// Conditions track the state of the LLMServerPool.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ReadyServers are the number of available servers within the LLMServerPool.
	ReadyServers int32 `json:"readyServers,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LLMServerPool is the Schema for the llmserverpools API
type LLMServerPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LLMServerPoolSpec   `json:"spec,omitempty"`
	Status LLMServerPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LLMServerPoolList contains a list of LLMServerPool
type LLMServerPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LLMServerPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LLMServerPool{}, &LLMServerPoolList{})
}
