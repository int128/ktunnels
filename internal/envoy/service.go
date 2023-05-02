package envoy

import (
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewService(key types.NamespacedName, tunnel ktunnelsv1.Tunnel) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "proxy",
					Port:       tunnel.Spec.Port,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: *tunnel.Status.TransitPort},
				},
			},
			Selector: map[string]string{
				PodLabelKeyOfProxy: tunnel.Spec.Proxy.Name,
			},
		},
	}
}
