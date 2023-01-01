package envoy

import (
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

const PodLabelKeyOfProxy = "ktunnels.int128.github.io/proxy"

func NewDeployment(key types.NamespacedName, proxy ktunnelsv1.Proxy, envoyImage string) appsv1.Deployment {
	if proxy.Spec.Template.Spec.Envoy.Image != "" {
		envoyImage = proxy.Spec.Template.Spec.Envoy.Image
	}

	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: proxy.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					PodLabelKeyOfProxy: proxy.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						PodLabelKeyOfProxy: proxy.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "envoy",
							Args:  []string{"-c", "/etc/envoy/bootstrap.json"},
							Image: envoyImage,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: pointer.Bool(false),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "envoy-config",
									MountPath: "/etc/envoy",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "envoy-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										// assume same name of ConfigMap and Deployment
										Name: key.Name,
									},
								},
							},
						},
					},
					ImagePullSecrets: proxy.Spec.Template.Spec.ImagePullSecrets,
				},
			},
		},
	}
}
