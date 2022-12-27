package controllers

import (
	"context"
	"time"

	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	"github.com/int128/ktunnels/pkg/envoy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("Proxy controller", func() {
	Context("When a Proxy is created", func() {
		It("Should create a Deployment and ConfigMap", func(ctx context.Context) {
			By("Creating a Proxy")
			proxy := ktunnelsv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: "default",
				},
			}
			Expect(k8sClient.Create(ctx, &proxy)).Should(Succeed())

			By("Getting the Deployment")
			var deployment appsv1.Deployment
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ktunnels-proxy-example",
					Namespace: "default",
				}, &deployment)
			}).Should(Succeed())

			Expect(deployment.Spec.Selector.MatchLabels).Should(Equal(map[string]string{
				envoy.PodLabelKeyOfProxy: "example",
			}))
			Expect(deployment.Spec.Template.Labels).Should(Equal(map[string]string{
				envoy.PodLabelKeyOfProxy: "example",
			}))
			Expect(deployment.Spec.Template.Spec.Volumes).Should(ContainElement(corev1.Volume{
				Name: "envoy-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "ktunnels-proxy-example"},
						DefaultMode:          pointer.Int32(420),
					},
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Args).Should(Equal([]string{"-c", "/etc/envoy/bootstrap.json"}))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).Should(Equal("envoyproxy/envoy:v1.22-latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(corev1.VolumeMount{
				Name:      "envoy-config",
				MountPath: "/etc/envoy",
			}))

			By("Getting the ConfigMap")
			var cm corev1.ConfigMap
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ktunnels-proxy-example",
					Namespace: "default",
				}, &cm)
			}).Should(Succeed())

			Expect(cm.Data).Should(HaveKey("bootstrap.json"))
			Expect(cm.Data).Should(HaveKey("cds.yaml"))
			Expect(cm.Data).Should(HaveKey("lds.yaml"))

		}, SpecTimeout(3*time.Second))
	})
})
