package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TunnelSpec defines the desired state of Tunnel
type TunnelSpec struct {
	// Destination hostname of this tunnel.
	Host string `json:"host,omitempty"`

	// Destination port of this tunnel.
	Port int32 `json:"port,omitempty"`

	// Proxy resource to register.
	Proxy corev1.LocalObjectReference `json:"proxy,omitempty"`
}

// TunnelStatus defines the observed state of Tunnel
type TunnelStatus struct {
	// Transit port of the proxy.
	// This value is automatically set by proxy controller. Do not set this manually.
	// +optional
	TransitPort *int32 `json:"transitPort,omitempty"`

	// True if the service is created.
	// +optional
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`
// +kubebuilder:printcolumn:name="Port",type=integer,JSONPath=`.spec.port`
// +kubebuilder:printcolumn:name="Proxy",type=string,JSONPath=`.spec.proxy.name`

// Tunnel is the Schema for the tunnels API
type Tunnel struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Tunnel
	// +required
	Spec TunnelSpec `json:"spec"`

	// status defines the observed state of Tunnel
	// +optional
	Status TunnelStatus `json:"status,omitzero"`
}

//+kubebuilder:object:root=true

// TunnelList contains a list of Tunnel
type TunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Tunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Tunnel{}, &TunnelList{})
		return nil
	})
}
