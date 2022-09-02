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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AutoVirtualDCSpec defines the desired state of AutoVirtualDC
type AutoVirtualDCSpec struct {
	// Template is a template for VirtualDC
	Template VirtualDC `json:"template,omitempty"`

	// StartSchedule is time to start VirtualDC. This format is cron format.
	StartSchedule string `json:"startSchedule,omitempty"`

	// StopSchedule is time to stop VirtualDC. this format is cron format.
	StopSchedule string `json:"stopSchedule,omitempty"`

	// TimeoutDuration is the duration of retry.  This format is format used by ParseDuration(https://pkg.go.dev/time#ParseDuration)
	TimeoutDuration string `json:"timeoutDuration,omitempty"`
}

// AutoVirtualDCStatus defines the observed state of AutoVirtualDC
type AutoVirtualDCStatus struct {
	// Next start time of VirtualDC's schedule.
	NextStartTime *metav1.Time `json:"nextStartTime,omitempty"`

	// Next stop time of VirtualDC's schedule.
	NextStopTime *metav1.Time `json:"nextStopTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AutoVirtualDC is the Schema for the autovirtualdcs API
type AutoVirtualDC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutoVirtualDCSpec   `json:"spec,omitempty"`
	Status AutoVirtualDCStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AutoVirtualDCList contains a list of AutoVirtualDC
type AutoVirtualDCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AutoVirtualDC `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AutoVirtualDC{}, &AutoVirtualDCList{})
}
