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

// InferenceModelSpec represents a set of Models/Adapters that are multiplexed onto one
// or more backend pools. This resource is managed by the "Inference Workload Owner"
// persona. The Inference Workload Owner persona is: a team that trains, verifies, and
// leverages a large language model from a model frontend, drives the lifecycle
// and rollout of new versions of those models, and defines the specific
// performance and latency goals for the model. These workloads are
// expected to operate within an InferencePool sharing compute capacity with other
// InferenceModels, defined by the Inference Platform Admin. We allow a user who
// has multiple InferenceModels across multiple pools (with the same config) to
// specify the configuration exactly once, and deploy to many pools
// simultaneously. Enabling a simpler config and single source of truth
// for a given user. InferenceModel names are unique for a given InferencePool,
// if the name is reused, an error will be  shown on the status of a
// InferenceModel that attempted to reuse. The oldest InferenceModel, based on
// creation timestamp, will be selected to remain valid. In the event of a race
// condition, one will be selected at random.
type InferenceModelSpec struct {
	// The name of the model as the users set in the "model" parameter in the requests.
	// The name should be unique among the workloads that reference the same backend pool.
	// This is the parameter that will be used to match the request with. In the future, we may
	// allow to match on other request parameters. The other approach to support matching on
	// on other request parameters is to use a different ModelName per HTTPFilter.
	// Names can be reserved without implementing an actual model in the pool.
	// This can be done by specifying a target model and setting the weight to zero,
	// an error will be returned specifying that no valid target model is found.
	ModelName string `json:"modelName,omitempty"`
	// Optional
	// Defines how important it is to serve the model compared to other models referencing the same pool.
	Criticality *Criticality `json:"criticality,omitempty"`
	// Optional.
	// Allow multiple versions of a model for traffic splitting.
	// If not specified, the target model name is defaulted to the modelName parameter.
	// modelName is often in reference to a LoRA adapter.
	TargetModels []TargetModel `json:"targetModels,omitempty"`
	// Reference to the InferencePool that the model registers to. It must exist in the same namespace.
	PoolRef LocalObjectReference `json:"poolRef,omitempty"`
}

// LocalObjectReference identifies an API object within the namespace of the
// referrer.
type LocalObjectReference struct {
	// Group is the group of the referent.
	Group string `json:"group,omitempty"`

	// Kind is kind of the referent. For example "InferencePool".
	Kind string `json:"kind,omitempty"`

	// Name is the name of the referent.
	Name string `json:"name,omitempty"`
}

// Defines how important it is to serve the model compared to other models.
type Criticality string

const (
	// Most important. Requests to this band will be shed last.
	Critical Criticality = "Critical"
	// More important than Sheddable, less important than Critical.
	// Requests in this band will be shed before critical traffic.
	Default Criticality = "Default"
	// Least important. Requests to this band will be shed before all other bands.
	Sheddable Criticality = "Sheddable"
)

// TargetModel represents a deployed model or a LoRA adapter. The
// Name field is expected to match the name of the LoRA adapter
// (or base model) as it is registered within the model server. Inference
// Gateway assumes that the model exists on the model server and is the
// responsibility of the user to validate a correct match. Should a model fail
// to exist at request time, the error is processed by the Instance Gateway,
// and then emitted on the appropriate InferenceModel object.
type TargetModel struct {
	// The name of the adapter as expected by the ModelServer.
	Name string `json:"name,omitempty"`
	// Weight is used to determine the percentage of traffic that should be
	// sent to this target model when multiple versions of the model are specified.
	Weight int `json:"weight,omitempty"`
}

// InferenceModelStatus defines the observed state of InferenceModel
type InferenceModelStatus struct {
	// Conditions track the state of the InferencePool.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// InferenceModel is the Schema for the InferenceModels API
type InferenceModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferenceModelSpec   `json:"spec,omitempty"`
	Status InferenceModelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InferenceModelList contains a list of InferenceModel
type InferenceModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferenceModel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferenceModel{}, &InferenceModelList{})
}
