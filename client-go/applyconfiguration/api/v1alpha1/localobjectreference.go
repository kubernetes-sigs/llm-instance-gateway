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

// LocalObjectReferenceApplyConfiguration represents a declarative configuration of the LocalObjectReference type for use
// with apply.
type LocalObjectReferenceApplyConfiguration struct {
	Group *string `json:"group,omitempty"`
	Kind  *string `json:"kind,omitempty"`
	Name  *string `json:"name,omitempty"`
}

// LocalObjectReferenceApplyConfiguration constructs a declarative configuration of the LocalObjectReference type for use with
// apply.
func LocalObjectReference() *LocalObjectReferenceApplyConfiguration {
	return &LocalObjectReferenceApplyConfiguration{}
}

// WithGroup sets the Group field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Group field is set to the value of the last call.
func (b *LocalObjectReferenceApplyConfiguration) WithGroup(value string) *LocalObjectReferenceApplyConfiguration {
	b.Group = &value
	return b
}

// WithKind sets the Kind field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Kind field is set to the value of the last call.
func (b *LocalObjectReferenceApplyConfiguration) WithKind(value string) *LocalObjectReferenceApplyConfiguration {
	b.Kind = &value
	return b
}

// WithName sets the Name field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Name field is set to the value of the last call.
func (b *LocalObjectReferenceApplyConfiguration) WithName(value string) *LocalObjectReferenceApplyConfiguration {
	b.Name = &value
	return b
}
