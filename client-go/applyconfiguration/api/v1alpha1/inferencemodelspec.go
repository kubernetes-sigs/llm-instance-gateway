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
// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "inference.networking.x-k8s.io/llm-instance-gateway/api/v1alpha1"
)

// InferenceModelSpecApplyConfiguration represents a declarative configuration of the InferenceModelSpec type for use
// with apply.
type InferenceModelSpecApplyConfiguration struct {
	ModelName    *string                                 `json:"modelName,omitempty"`
	Criticality  *v1alpha1.Criticality                   `json:"criticality,omitempty"`
	TargetModels []TargetModelApplyConfiguration         `json:"targetModels,omitempty"`
	PoolRef      *LocalObjectReferenceApplyConfiguration `json:"poolRef,omitempty"`
}

// InferenceModelSpecApplyConfiguration constructs a declarative configuration of the InferenceModelSpec type for use with
// apply.
func InferenceModelSpec() *InferenceModelSpecApplyConfiguration {
	return &InferenceModelSpecApplyConfiguration{}
}

// WithModelName sets the ModelName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ModelName field is set to the value of the last call.
func (b *InferenceModelSpecApplyConfiguration) WithModelName(value string) *InferenceModelSpecApplyConfiguration {
	b.ModelName = &value
	return b
}

// WithCriticality sets the Criticality field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Criticality field is set to the value of the last call.
func (b *InferenceModelSpecApplyConfiguration) WithCriticality(value v1alpha1.Criticality) *InferenceModelSpecApplyConfiguration {
	b.Criticality = &value
	return b
}

// WithTargetModels adds the given value to the TargetModels field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the TargetModels field.
func (b *InferenceModelSpecApplyConfiguration) WithTargetModels(values ...*TargetModelApplyConfiguration) *InferenceModelSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithTargetModels")
		}
		b.TargetModels = append(b.TargetModels, *values[i])
	}
	return b
}

// WithPoolRef sets the PoolRef field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PoolRef field is set to the value of the last call.
func (b *InferenceModelSpecApplyConfiguration) WithPoolRef(value *LocalObjectReferenceApplyConfiguration) *InferenceModelSpecApplyConfiguration {
	b.PoolRef = value
	return b
}
