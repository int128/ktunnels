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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProxySpec defines the desired state of Proxy
type ProxySpec struct {
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// +optional
	Template ProxyPod `json:"template,omitempty"`
}

// ProxyPod defines the desired state of a Pod
type ProxyPod struct {
	// +optional
	Spec ProxyPodSpec `json:"spec,omitempty"`
}

// ProxyPodSpec defines the desired state of a Pod
type ProxyPodSpec struct {
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// +optional
	Envoy        ProxyEnvoy        `json:"envoy,omitempty"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// ProxyEnvoy defines the desired state of an Envoy container
type ProxyEnvoy struct {
	// Envoy image tag.
	// Default to the image shipped with the controller.
	// +optional
	Image *string `json:"image,omitempty"`

	// Resource requirements.
	// Default to the suitable value.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ProxyStatus defines the observed state of Proxy
type ProxyStatus struct {
	// Ready becomes true when the owned Deployment is ready
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`

// Proxy is the Schema for the proxies API
type Proxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxySpec   `json:"spec,omitempty"`
	Status ProxyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProxyList contains a list of Proxy
type ProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Proxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Proxy{}, &ProxyList{})
}
