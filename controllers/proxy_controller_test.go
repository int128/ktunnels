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
	var proxy ktunnelsv1.Proxy
	var tunnel ktunnelsv1.Tunnel
	BeforeEach(func(ctx context.Context) {
		By("Creating a Proxy")
		proxy = ktunnelsv1.Proxy{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "example-",
				Namespace:    "default",
			},
		}
		Expect(k8sClient.Create(ctx, &proxy)).Should(Succeed())

		By("Creating a tunnel")
		tunnel = ktunnelsv1.Tunnel{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "microservice-database-",
				Namespace:    "default",
			},
			Spec: ktunnelsv1.TunnelSpec{
				Host:  "microservice-database.staging",
				Port:  5432,
				Proxy: corev1.LocalObjectReference{Name: proxy.Name},
			},
		}
		Expect(k8sClient.Create(ctx, &tunnel)).Should(Succeed())
	})

	Context("When a Proxy is created", func() {
		It("Should create a Deployment and ConfigMap", func(ctx context.Context) {
			By("Getting the Deployment")
			var deployment appsv1.Deployment
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ktunnels-proxy-" + proxy.Name,
					Namespace: "default",
				}, &deployment)
			}).Should(Succeed())

			Expect(deployment.Spec.Selector.MatchLabels).Should(Equal(map[string]string{
				envoy.PodLabelKeyOfProxy: proxy.Name,
			}))
			Expect(deployment.Spec.Template.Labels).Should(Equal(map[string]string{
				envoy.PodLabelKeyOfProxy: proxy.Name,
			}))
			Expect(deployment.Spec.Template.Spec.Volumes).Should(ContainElement(corev1.Volume{
				Name: "envoy-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "ktunnels-proxy-" + proxy.Name},
						DefaultMode:          pointer.Int32(420),
					},
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Args).Should(Equal([]string{"-c", "/etc/envoy/bootstrap.json"}))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).Should(Equal("envoyproxy/envoy:v0.00-latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(corev1.VolumeMount{
				Name:      "envoy-config",
				MountPath: "/etc/envoy",
			}))

			By("Getting the ConfigMap")
			var cm corev1.ConfigMap
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ktunnels-proxy-" + proxy.Name,
					Namespace: "default",
				}, &cm)
			}).Should(Succeed())

			Expect(cm.Data).Should(HaveKey("bootstrap.json"))
			Expect(cm.Data).Should(HaveKey("cds.json"))
			Expect(cm.Data).Should(HaveKey("lds.json"))

		}, SpecTimeout(3*time.Second))

		It("Should update the status of Tunnel", func(ctx context.Context) {
			By("Verifying the Tunnel status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      tunnel.Name,
					Namespace: tunnel.Namespace,
				}, &tunnel)).Should(Succeed())
				g.Expect(tunnel.Status.TransitPort).ShouldNot(BeNil())
			}).Should(Succeed())

		}, SpecTimeout(3*time.Second))

		It("Should update the status of Proxy", func(ctx context.Context) {
			By("Verifying the status of Proxy")
			Expect(proxy.Status.Ready).Should(BeFalse())

			By("Updating the status of Deployment")
			var deployment appsv1.Deployment
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ktunnels-proxy-" + proxy.Name,
					Namespace: "default",
				}, &deployment)
			}).Should(Succeed())
			deployment.Status.Replicas = 1
			deployment.Status.ReadyReplicas = 1
			deployment.Status.AvailableReplicas = 1
			Expect(k8sClient.Status().Update(ctx, &deployment)).Should(Succeed())

			By("Verifying the status of Proxy")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      proxy.Name,
					Namespace: proxy.Namespace,
				}, &proxy)).Should(Succeed())
				g.Expect(proxy.Status.Ready).Should(BeTrue())
			}).Should(Succeed())

		}, SpecTimeout(3*time.Second))
	})
})
