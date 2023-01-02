package envoy

import (
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const PodLabelKeyOfProxy = "ktunnels.int128.github.io/proxy"

func NewDeployment(key types.NamespacedName, proxy ktunnelsv1.Proxy) appsv1.Deployment {
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
							Name: "envoy",
							Args: []string{"-c", "/etc/envoy/bootstrap.json"},
							Image: mergeValue(
								DefaultImage,
								proxy.Spec.Template.Spec.Envoy.Image,
							),
							Resources: mergeValue(
								corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
								},
								proxy.Spec.Template.Spec.Envoy.Resources,
							),
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										// https://www.envoyproxy.io/docs/envoy/latest/operations/admin#get--ready
										Path:   "/ready",
										Port:   intstr.FromInt(9901),
										Scheme: "HTTP",
									},
								},
								TimeoutSeconds:      1,
								PeriodSeconds:       5,
								SuccessThreshold:    1,
								FailureThreshold:    3,
								InitialDelaySeconds: 1,
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

func mergeValue[T any](defaultValue T, overrides ...*T) T {
	for i := len(overrides) - 1; i >= 0; i-- {
		if overrides[i] != nil {
			return *overrides[i]
		}
	}
	return defaultValue
}
