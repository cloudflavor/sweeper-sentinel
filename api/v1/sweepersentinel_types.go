/*
Copyright 2025 Cloudflavor.

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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SweeperSentinelSpec defines the desired state of SweeperSentinel
type SweeperSentinelSpec struct {
	// Targets - a user defined list of GVK to prune
	Targets []Target `json:"targets"`
}

type Target struct {
	// +kubebuilder:validation:MinLength=2
	// +required
	APIVersion string `json:"apiVersion"`

	// +required
	Kind string `json:"kind"`
}

// SweeperSentinelStatus defines the observed state of SweeperSentinel.
type SweeperSentinelStatus struct {
	// active defines a list of pointers to currently running jobs.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// lastScheduleTime defines when was the last time the job was successfully scheduled.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SweeperSentinel is the Schema for the sweepersentinels API
type SweeperSentinel struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SweeperSentinel
	// +required
	Spec SweeperSentinelSpec `json:"spec"`

	// status defines the observed state of SweeperSentinel
	// +optional
	Status SweeperSentinelStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SweeperSentinelList contains a list of SweeperSentinel
type SweeperSentinelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SweeperSentinel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SweeperSentinel{}, &SweeperSentinelList{})
}
