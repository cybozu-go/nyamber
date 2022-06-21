/*
Copyright 2022.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VirtualDCSpec defines the desired state of VirtualDC
type VirtualDCSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Neco branch is a target branch name for dctest
	//+kubebuiler:validation:Optional
	//+kubebuilder:default=main
	NecoBranch string `json:"necoBranch,omitempty"`

	// Neco apps branch is a target branch name for dctest
	//+kubebuiler:validation:Optional
	//+kubebuilder:default=main
	NecoAppsBranch string `json:"necoAppsBranch,omitempty"`

	// Command is run after creating dctest pods
	//+kubebuiler:validation:Optional
	Command []string `json:"command,omitempty"`

	//+kubebuiler:validation:Optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Volume for ConfigMap
}

// VirtualDCStatus defines the observed state of VirtualDC
type VirtualDCStatus struct {
	// Conditions is an array of conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	PodCreated   string = "PodCreated"
	PodAvailable string = "PodAvailable"
)

const (
	PodCreatedAlreadyExists  string = "AlreadyExists"
	PodCreatedFailed         string = "Failed"
	PodAvailableNotScheduled string = "NotScheduled"
	PodAvailableNotAvailable string = "NotAvailable"
	PodAvailableNotExists    string = "NotExists"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VirtualDC is the Schema for the virtualdcs API
type VirtualDC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualDCSpec   `json:"spec,omitempty"`
	Status VirtualDCStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VirtualDCList contains a list of VirtualDC
type VirtualDCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualDC `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualDC{}, &VirtualDCList{})
}
