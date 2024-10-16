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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LLMService represents a set of LLM services that are multiplexed onto one
// or more backend pools. This resource is managed by the "LLM Service Owner"
// persona. The Service Owner persona is: a team that trains, verifies, and
// leverages a large language model from a model frontend, drives the lifecycle
// and rollout of new versions of those models, and defines the specific
// performance and latency goals for the model. These services are
// expected to operate within a LLMServerPool sharing compute capacity with other
// LLMServices, defined by the Inference Platform Admin. We allow a user who
// has multiple LLMServices across multiple pools (with the same config) to
// specify the configuration exactly once, and deploy to many pools
// simultaneously. Enabling a simpler config and single source of truth
// for a given user. LLMService names are unique for a given LLMServerPool,
// if the name is reused, an error will be  shown on the status of a
// LLMService that attempted to reuse. The oldest LLMService, based on
// creation timestamp, will be selected to remain valid. In the event of a race
// condition, one will be selected at random.
type LLMServiceSpec struct {
	// Model defines the distinct services.
	// Model can be in 2 priority classes, Critical and Noncritical.
	// Priority class is implicitly set to Critical by specifying an Objective.
	// Otherwise the Model is considered Noncritical.
	Models []Model `json:"models,omitempty"`
	// PoolRef are references to the backend pools that the LLMService registers to.
	PoolRef []corev1.ObjectReference `json:"poolRef,omitempty"`
}

// Model defines the policies for routing the traffic of a use case, this includes performance objectives
// and traffic splitting between different versions of the model.
type Model struct {
	// The name of the model as the users set in the "model" parameter in the requests.
	// The name should be unique among the services that reference the same backend pool.
	// This is the parameter that will be used to match the request with. In the future, we may
	// allow to match on other request parameters. The other approach to support matching on
	// on other request parameters is to use a different ModelName per HTTPFilter.
	// Names can be reserved without implementing an actual model in the pool.
	// This can be done by specifying a target model and setting the weight to zero,
	// an error will be returned specifying that no valid target model is found.
	Name string `json:"name,omitempty"`
	// Optional
	// LLM Services with an objective have higher priority than services without.
	// IMPORTANT: By specifying an objective, this places the LLMService in a higher priority class than LLMServices without a defined priority class.
	// In the face of resource-scarcity. Higher priority requests will be preserved, and lower priority class requests will be rejected.
	Objective *Objective `json:"objective,omitempty"`
	// Optional.
	// Allow multiple versions of a model for traffic splitting.
	// If not specified, the target model name is defaulted to the modelName parameter.
	// modelName is often in reference to a LoRA adapter.
	TargetModels []TargetModel `json:"targetModels,omitempty"`
}

// TargetModel represents a deployed model or a LoRA adapter. The
// Name field is expected to match the name of the LoRA adapter
// (or base model) as it is registered within the model server. Inference
// Gateway assumes that the model exists on the model server and is the
// responsibility of the user to validate a correct match. Should a model fail
// to exist at request time, the error is processed by the Instance Gateway,
// and then emitted on the appropriate LLMService object.
type TargetModel struct {
	// The name of the adapter as expected by the ModelServer.
	Name string `json:"name,omitempty"`
	// Weight is used to determine the percentage of traffic that should be
	// sent to this target model when multiple versions of the model are specified.
	Weight int `json:"weight,omitempty"`
}

// Objective captures the latency SLO of a LLM service.
// In MVP, meeting the SLO is on a best effort basis.
// Future: Extend the API for different behaviors of meeting the SLO.
// The gateway will perform best-effort load balancing, and work with other components (e.g., autoscaler) to meet the
// objectives.
type Objective struct {
	// The AverageLatencyPerOutputToken is calculated as the e2e request latency divided by output token
	// length. Note that this is different from what is known as TPOT (time per output token) which only
	// takes decode time into account.
	// The P95 is calculated over a fixed time window defined at the operator level.
	DesiredAveragePerOutputTokenLatencyAtP95OverMultipleRequests *time.Duration `json:"desiredAveragePerOutputTokenLatencyAtP95OverMultipleRequests,omitempty"`
}

// LLMServiceStatus defines the observed state of LLMService
type LLMServiceStatus struct {
	// Conditions track the state of the LLMServerPool.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient

// LLMService is the Schema for the llmservices API
type LLMService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LLMServiceSpec   `json:"spec,omitempty"`
	Status LLMServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LLMServiceList contains a list of LLMService
type LLMServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LLMService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LLMService{}, &LLMServiceList{})
}
