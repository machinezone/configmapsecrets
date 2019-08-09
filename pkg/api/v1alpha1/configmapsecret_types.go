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
	// +optional
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
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// List of ConfigMapSecrets.
	Items []ConfigMapSecret `json:"items"`
}

// ConfigMapSecretSpec defines the desired state of a ConfigMapSecret.
type ConfigMapSecretSpec struct {
	// Template that describes the config that will be rendered.
	// Variable references $(VAR_NAME) in template data are expanded using the
	// ConfigMapSecret's variables. If a variable cannot be resolved, the reference
	// in the input data will be unchanged. The $(VAR_NAME) syntax can be escaped
	// with a double $$, ie: $$(VAR_NAME). Escaped references will never be expanded,
	// regardless of whether the variable exists or not.
	// +optional
	Template ConfigMapTemplate `json:"template,omitempty"`

	// List of template variables.
	// +optional
	Vars []TemplateVariable `json:"vars,omitempty"`
}

// ConfigMapTemplate is a ConfigMap template.
type ConfigMapTemplate struct {
	// Standard object metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Data contains the configuration data.
	// Each key must consist of alphanumeric characters, '-', '_' or '.'.
	// Values with non-UTF-8 byte sequences must use the BinaryData field.
	// The keys stored in Data must not overlap with the keys in
	// the BinaryData field.
	// +optional
	Data map[string]string `json:"data,omitempty"`

	// BinaryData contains the binary data.
	// Each key must consist of alphanumeric characters, '-', '_' or '.'.
	// BinaryData can contain byte sequences that are not in the UTF-8 range.
	// The keys stored in BinaryData must not overlap with the keys in
	// the Data field.
	// +optional
	BinaryData map[string][]byte `json:"binaryData,omitempty"`
}

// TemplateVariable is a template variable.
type TemplateVariable struct {
	// Name of the template variable.
	Name string `json:"name"`

	// Variable references $(VAR_NAME) are expanded using the previous defined
	// environment variables in the ConfigMapSecret. If a variable cannot be resolved,
	// the reference in the input string will be unchanged. The $(VAR_NAME) syntax
	// can be escaped with a double $$, ie: $$(VAR_NAME). Escaped references will
	// never be expanded, regardless of whether the variable exists or not.
	// Defaults to "".
	// +optional
	Value string `json:"value,omitempty"`
	// SecretValue selects a value by its key in a Secret.
	// +optional
	SecretValue *corev1.SecretKeySelector `json:"secretValue,omitempty"`
	// ConfigMapValue selects a value by its key in a ConfigMap.
	// +optional
	ConfigMapValue *corev1.ConfigMapKeySelector `json:"configMapValue,omitempty"`
}

// ConfigMapSecretStatus describes the observed state of a ConfigMapSecret.
type ConfigMapSecretStatus struct {
	// The generation observed by the ConfigMapSecret controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of a ConfigMapSecret's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []ConfigMapSecretCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// ConfigMapSecretCondition describes the state of a ConfigMapSecret.
type ConfigMapSecretCondition struct {
	// Type of the condition.
	Type ConfigMapSecretConditionType `json:"type"`
	// Status of the condition: True, False, or Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time the condition was updated.
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the last update.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the last update.
	// +optional
	Message string `json:"message,omitempty"`
}

// ConfigMapSecretConditionType is a valid value for ConfigMapSecretCondition.Type
type ConfigMapSecretConditionType string

const (
	// ConfigMapSecretRenderFailure means that the target secret could not be
	// rendered.
	ConfigMapSecretRenderFailure ConfigMapSecretConditionType = "RenderFailure"
)
