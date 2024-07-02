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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FlipPhase string

const (
	// FlipPending means the Flip has not started yet
	FlipPending FlipPhase = "Pending"
	// FlipRunning means the Flip is running
	FlipRunning FlipPhase = "Running"
	// FlipFailed means the Flip has failed
	FlipFailed FlipPhase = "Failed"
	// FlipSucceeded means the Flip has succeeded
	FlipSucceeded FlipPhase = "Succeeded"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FlipperSpec defines the desired state of Flipper
type FlipperSpec struct {

	//Interval of rollout restart of deployment by default it is 10m
	//See https://pkg.go.dev/time#ParseDuration for more information
	Interval string `json:"interval,omitempty"`
	//Match is used to filter the deployments
	Match MatchFilter `json:"match,omitempty"`
}

type MatchFilter struct {
	//Labels to is used to filter the deployments
	Labels map[string]string `json:"labels,omitempty"`
	//Namespace in which  to filter the deployments
	Namespace string `json:"namespace,omitempty"`
}

// FlipperStatus defines the observed state of Flipper
type FlipperStatus struct {

	// Represents the whether rollout restart was triggered
	Phase FlipPhase `json:"status"`

	// Reason for rolloutRestart failure
	Reason string `json:"reason,omitempty"`

	// List of deployments that failed to get patched (namespacedName)
	FailedRolloutDeployments []appsv1.Deployment `json:"failedRolloutDeployments,omitempty"`

	// Time of the last rollout restart
	LastScheduledRolloutTime metav1.Time `json:"lastScheduleTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Flipper is the Schema for the flippers API
// +kubebuilder:subresource:status
type Flipper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlipperSpec   `json:"spec,omitempty"`
	Status FlipperStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FlipperList contains a list of Flipper
type FlipperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Flipper `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Flipper{}, &FlipperList{})
}
