// Copyright 2019 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ConfigMapSecret{}, &ConfigMapSecretList{})
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ConfigMapSecret holds configuration data with embedded secrets.
type ConfigMapSecret struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Desired state of the ConfigMapSecret.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec ConfigMapSecretSpec `json:"spec,omitempty"`

	// Observed state of the ConfigMapSecret.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status ConfigMapSecretStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigMapSecretList contains a list of ConfigMapSecrets.
type ConfigMapSecretList struct {
	metav1.TypeMeta `json:",inline"`

	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#lists-and-simple-kinds
	metav1.ListMeta `json:"metadata,omitempty"`

	// List of ConfigMapSecrets.
	Items []ConfigMapSecret `json:"items"`
}

// ConfigMapSecretSpec defines the desired state of a ConfigMapSecret.
type ConfigMapSecretSpec struct {
	// Template that describes the config that will be rendered.
	//
	// Variable references $(VAR_NAME) in template data are expanded using the
	// ConfigMapSecret's variables. If a variable cannot be resolved, the reference
	// in the input data will be unchanged. The $(VAR_NAME) syntax can be escaped
	// with a double $$, ie: $$(VAR_NAME). Escaped references will never be expanded,
	// regardless of whether the variable exists or not.
	Template ConfigMapTemplate `json:"template,omitempty"`

	// List of sources to populate template variables.
	// Keys defined in a source must consist of alphanumeric characters, '-', '_' or '.'.
	// When a key exists in multiple sources, the value associated with the last
	// source will take precedence. Values defined by Vars with a duplicate key
	// will take precedence.
	VarsFrom []VarsFromSource `json:"varsFrom,omitempty"`

	// List of template variables.
	Vars []Var `json:"vars,omitempty"`
}

// ConfigMapTemplate is a ConfigMap template.
type ConfigMapTemplate struct {
	// Metadata is a stripped down version of the standard object metadata.
	// Its properties will be applied to the metadata of the generated Secret.
	// If no name is provided, the name of the ConfigMapSecret will be used.
	Metadata EmbeddedObjectMeta `json:"metadata,omitempty"`

	// Data contains the configuration data.
	// Each key must consist of alphanumeric characters, '-', '_' or '.'.
	// Values with non-UTF-8 byte sequences must use the BinaryData field.
	// The keys stored in Data must not overlap with the keys in
	// the BinaryData field.
	Data map[string]string `json:"data,omitempty"`

	// BinaryData contains the binary data.
	// Each key must consist of alphanumeric characters, '-', '_' or '.'.
	// BinaryData can contain byte sequences that are not in the UTF-8 range.
	// The keys stored in BinaryData must not overlap with the keys in
	// the Data field.
	BinaryData map[string][]byte `json:"binaryData,omitempty"`
}

// EmbeddedObjectMeta contains a subset of the fields from k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta.
// Only fields which are relevant to embedded resources are included.
type EmbeddedObjectMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// More info: https://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/user-guide/labels
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/user-guide/annotations
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Var is a template variable.
type Var struct {
	// Name of the template variable.
	Name string `json:"name"`

	// Variable references $(VAR_NAME) are expanded using the previous defined
	// environment variables in the ConfigMapSecret. If a variable cannot be resolved,
	// the reference in the input string will be unchanged. The $(VAR_NAME) syntax
	// can be escaped with a double $$, ie: $$(VAR_NAME). Escaped references will
	// never be expanded, regardless of whether the variable exists or not.
	Value string `json:"value,omitempty"`

	// SecretValue selects a value by its key in a Secret.
	SecretValue *corev1.SecretKeySelector `json:"secretValue,omitempty"`

	// ConfigMapValue selects a value by its key in a ConfigMap.
	ConfigMapValue *corev1.ConfigMapKeySelector `json:"configMapValue,omitempty"`
}

// VarsFromSource represents the source of a set of template variables.
type VarsFromSource struct {
	// An optional identifier to prepend to each key.
	Prefix string `json:"prefix,omitempty"`

	// The Secret to select.
	SecretRef *SecretVarsSource `json:"secretRef,omitempty"`

	// The ConfigMap to select.
	ConfigMapRef *ConfigMapVarsSource `json:"configMapRef,omitempty"`
}

// SecretVarsSource selects a Secret to populate template variables with.
type SecretVarsSource struct {
	// The Secret to select.
	corev1.LocalObjectReference `json:",inline"`

	// Specify whether the Secret must be defined.
	Optional *bool `json:"optional,omitempty"`
}

// ConfigMapVarsSource selects a ConfigMap to populate template variables with.
type ConfigMapVarsSource struct {
	// The ConfigMap to select.
	corev1.LocalObjectReference `json:",inline"`

	// Specify whether the ConfigMap must be defined.
	Optional *bool `json:"optional,omitempty"`
}

// ConfigMapSecretStatus describes the observed state of a ConfigMapSecret.
type ConfigMapSecretStatus struct {
	// The generation observed by the ConfigMapSecret controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of a ConfigMapSecret's current state.
	//
	// +listType=map
	// +listMapKey=type
	// +listMapKeys=type
	Conditions []ConfigMapSecretCondition `json:"conditions,omitempty"`
}

// ConfigMapSecretCondition describes the state of a ConfigMapSecret.
type ConfigMapSecretCondition struct {
	// Type of the condition.
	Type ConfigMapSecretConditionType `json:"type"`

	// Status of the condition: True, False, or Unknown.
	Status corev1.ConditionStatus `json:"status"`

	// The last time the condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// The reason for the last update.
	Reason string `json:"reason,omitempty"`

	// A human readable message indicating details about the last update.
	Message string `json:"message,omitempty"`
}

// ConfigMapSecretConditionType is a valid value for ConfigMapSecretCondition.Type
type ConfigMapSecretConditionType string

const (
	// ConfigMapSecretRenderFailure means that the target secret could not be
	// rendered.
	ConfigMapSecretRenderFailure ConfigMapSecretConditionType = "RenderFailure"
)
