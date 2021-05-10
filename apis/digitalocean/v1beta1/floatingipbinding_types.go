/*
Copyright 2021 Alex Williams.

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

type NodeSelectorPolicy string

const (
	Newest NodeSelectorPolicy = "Newest"
	Oldest NodeSelectorPolicy = "Oldest"
	Random NodeSelectorPolicy = "Random"
)

// FloatingIPBindingSpec defines the desired state of FloatingIPBinding
type FloatingIPBindingSpec struct {
	// The floating IP address to bind nodes to. i.e. "1.2.3.4"
	FloatingIP string `json:"floatingIP"`

	// An optional LabelSelector to select nodes. Defaults to all nodes.
	// A label selector is a label query over a set of resources. The result of matchLabels
	// and matchExpressions are ANDed. An empty label selector matches all objects. A null
	// label selector matches no objects.
	// +optional
	// +nullable
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// An optional policy to choose a node from those that match the NodeSelector
	// Defaults to Newest
	// +kubebuilder:default:="Newest"
	// +optional
	NodeSelectorPolicy NodeSelectorPolicy `json:"nodeSelectorPolicy,omitempty"`
}

// FloatingIPBindingStatus defines the observed state of FloatingIPBinding
type FloatingIPBindingStatus struct {
	AssignedDropletID   int    `json:"assignedDropletID,omitempty"`
	AssignedDropletName string `json:"assignedDropletName,omitempty"`
}

// +kubebuilder:object:root=true

// FloatingIPBinding is the Schema for the floatingipbindings API
// +kubebuilder:printcolumn:name="FLOATING_IP",type=string,JSONPath=`.spec.floatingIP`
// +kubebuilder:printcolumn:name="ASSIGNED_DROPLET_ID",type=string,JSONPath=`.status.assignedDropletID`
// +kubebuilder:printcolumn:name="ASSIGNED_DROPLET_NAME",type=string,JSONPath=`.status.assignedDropletName`
// +kubebuilder:subresource:status
type FloatingIPBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FloatingIPBindingSpec   `json:"spec,omitempty"`
	Status FloatingIPBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FloatingIPBindingList contains a list of FloatingIPBinding
type FloatingIPBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FloatingIPBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FloatingIPBinding{}, &FloatingIPBindingList{})
}
