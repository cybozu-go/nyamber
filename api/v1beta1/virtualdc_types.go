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

// VirtualDCSpec defines the desired state of VirtualDC
type VirtualDCSpec struct {
	// Neco branch to use for dctest.
	// If this field is empty, controller runs dctest with "main" branch
	//+kubebuiler:validation:Optional
	NecoBranch string `json:"necoBranch,omitempty"`

	// Neco-apps branch to use for dctest.
	// If this field is empty, controller runs dctest with "main" branch
	//+kubebuiler:validation:Optional
	NecoAppsBranch string `json:"necoAppsBranch,omitempty"`

	// Skip bootstrapping neco-apps if true
	//+kubebuilder:validation:Optional
	SkipNecoApps bool `json:"skipNecoApps,omitempty"`

	// Path to a user-defined script and its arguments to run after bootstrapping dctest
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
	TypePodCreated      string = "PodCreated"
	TypePodAvailable    string = "PodAvailable"
	TypeServiceCreated  string = "ServiceCreated"
	TypePodJobCompleted string = "PodJobCompleted"
)

const ReasonOK string = "OK"

const (
	ReasonPodCreatedConflict       string = "Conflict"
	ReasonPodCreatedFailed         string = "Failed"
	ReasonPodCreatedTemplateError  string = "TemplateError"
	ReasonPodAvailableNotAvailable string = "NotAvailable"
	ReasonPodAvailableNotExists    string = "NotExists"
	ReasonPodAvailableNotScheduled string = "NotScheduled"
	ReasonServiceCreatedConflict   string = "Conflict"
	ReasonServiceCreatedFailed     string = "Failed"
	ReasonPodJobCompletedPending   string = "Pending"
	ReasonPodJobCompletedRunning   string = "Running"
	ReasonPodJobCompletedFailed    string = "Failed"
)

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=vdc
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="PODAVAILABLE",type="string",JSONPath=".status.conditions[?(@.type=='PodAvailable')].status"
//+kubebuilder:printcolumn:name="JOBSTATUS",type="string",JSONPath=".status.conditions[?(@.type=='PodJobCompleted')].reason"
//+kubebuilder:printcolumn:name="JOBNAME",type="string",JSONPath=".status.conditions[?(@.type=='PodJobCompleted')].message"

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
