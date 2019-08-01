// Copyright 2019 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"github.com/machinezone/configmapsecrets/pkg/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// CreateVariablesErrorReason is the reason given when required ConfigMapSecret
	// variables cannot be resolved.
	CreateVariablesErrorReason = "CreateVariablesError"
)

// NewConfigMapSecretCondition creates a new deployment condition.
func NewConfigMapSecretCondition(typ v1alpha1.ConfigMapSecretConditionType, status corev1.ConditionStatus, reason, message string) *v1alpha1.ConfigMapSecretCondition {
	return &v1alpha1.ConfigMapSecretCondition{
		Type:               typ,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetConfigMapSecretCondition returns the condition with the provided type.
func GetConfigMapSecretCondition(status v1alpha1.ConfigMapSecretStatus, typ v1alpha1.ConfigMapSecretConditionType) *v1alpha1.ConfigMapSecretCondition {
	for _, c := range status.Conditions {
		if c.Type == typ {
			return &c
		}
	}
	return nil
}

// SetConfigMapSecretCondition updates the status to include the provided condition.
// If the condition already exists with the same status, reason, and message then it is not updated.
func SetConfigMapSecretCondition(status *v1alpha1.ConfigMapSecretStatus, cond v1alpha1.ConfigMapSecretCondition) {
	if prev := GetConfigMapSecretCondition(*status, cond.Type); prev != nil {
		if prev.Status == cond.Status &&
			prev.Reason == cond.Reason &&
			prev.Message == cond.Message {
			return
		}
		// Do not update lastTransitionTime if the status of the condition doesn't change.
		if prev.Status == cond.Status {
			cond.LastTransitionTime = prev.LastTransitionTime
		}
	}
	RemoveConfigMapSecretCondition(status, cond.Type)
	status.Conditions = append(status.Conditions, cond)
}

// RemoveConfigMapSecretCondition removes the condition with the provided type.
func RemoveConfigMapSecretCondition(status *v1alpha1.ConfigMapSecretStatus, typ v1alpha1.ConfigMapSecretConditionType) {
	var conds []v1alpha1.ConfigMapSecretCondition
	for _, c := range status.Conditions {
		if c.Type == typ {
			continue
		}
		conds = append(conds, c)
	}
	status.Conditions = conds
}
