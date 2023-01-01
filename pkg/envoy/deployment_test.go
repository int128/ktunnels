package envoy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

func TestNewDeployment(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		got := NewDeployment(
			types.NamespacedName{Namespace: "default", Name: "ktunnels-proxy-example"},
			ktunnelsv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "example",
				},
			},
			"envoyproxy/envoy:v0.00-latest",
		)
		want := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "ktunnels-proxy-example",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						PodLabelKeyOfProxy: "example",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							PodLabelKeyOfProxy: "example",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:      "envoy",
								Args:      []string{"-c", "/etc/envoy/bootstrap.json"},
								Image:     "envoyproxy/envoy:v0.00-latest",
								Resources: corev1.ResourceRequirements{},
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
											Name: "ktunnels-proxy-example",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("deployment mismatch mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("with full options", func(t *testing.T) {
		got := NewDeployment(
			types.NamespacedName{Namespace: "default", Name: "ktunnels-proxy-example"},
			ktunnelsv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "example",
				},
				Spec: ktunnelsv1.ProxySpec{
					Replicas: pointer.Int32(2),
					Template: ktunnelsv1.ProxyPod{
						Spec: ktunnelsv1.ProxyPodSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{{Name: "docker-hub"}},
							Envoy: ktunnelsv1.ProxyEnvoy{
								Image: "1234567890.dkr.ecr.us-east-1.amazonaws.com/envoy:v9.99",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
								},
							},
						},
					},
				},
			},
			"envoyproxy/envoy:v0.00-latest",
		)
		want := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "ktunnels-proxy-example",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						PodLabelKeyOfProxy: "example",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							PodLabelKeyOfProxy: "example",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "envoy",
								Args:  []string{"-c", "/etc/envoy/bootstrap.json"},
								Image: "1234567890.dkr.ecr.us-east-1.amazonaws.com/envoy:v9.99",
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
											Name: "ktunnels-proxy-example",
										},
									},
								},
							},
						},
						ImagePullSecrets: []corev1.LocalObjectReference{{Name: "docker-hub"}},
					},
				},
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("deployment mismatch mismatch (-want +got):\n%s", diff)
		}
	})
}
