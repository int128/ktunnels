package controllers

import (
	"context"
	"time"

	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Tunnel controller", func() {
	var proxy ktunnelsv1.Proxy
	BeforeEach(func(ctx context.Context) {
		By("Creating a proxy")
		proxy = ktunnelsv1.Proxy{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "example-",
				Namespace:    "default",
			},
		}
		Expect(k8sClient.Create(ctx, &proxy)).Should(Succeed())
	})

	Context("When a tunnel is created", func() {
		It("Should create a service", func(ctx context.Context) {
			By("Creating a tunnel")
			tunnel := ktunnelsv1.Tunnel{
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

			By("Getting the service")
			var svc corev1.Service
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      tunnel.Name,
					Namespace: "default",
				}, &svc)
			}).Should(Succeed())
			Expect(svc.Spec.Type).Should(Equal(corev1.ServiceTypeClusterIP))

			By("Verifying the status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      tunnel.Name,
					Namespace: tunnel.Namespace,
				}, &tunnel)).Should(Succeed())
				g.Expect(tunnel.Status.TransitPort).ShouldNot(BeNil())
				g.Expect(tunnel.Status.Ready).Should(BeTrue())
			}).Should(Succeed())
		}, SpecTimeout(3*time.Second))
	})

	Context("When a tunnel is created without proxy", func() {
		It("Should not be ready", func(ctx context.Context) {
			By("Creating a tunnel")
			tunnel := ktunnelsv1.Tunnel{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "microservice-database-",
					Namespace:    "default",
				},
				Spec: ktunnelsv1.TunnelSpec{
					Host:  "microservice-database.staging",
					Port:  5432,
					Proxy: corev1.LocalObjectReference{Name: "dummy-should-not-exist"},
				},
			}
			Expect(k8sClient.Create(ctx, &tunnel)).Should(Succeed())

			By("Verifying the status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      tunnel.Name,
					Namespace: tunnel.Namespace,
				}, &tunnel)).Should(Succeed())
				g.Expect(tunnel.Status.TransitPort).Should(BeNil())
				g.Expect(tunnel.Status.Ready).Should(BeFalse())
			}).Should(Succeed())
		}, SpecTimeout(3*time.Second))

		It("Should delete the service", func(ctx context.Context) {
			By("Creating a tunnel")
			tunnel := ktunnelsv1.Tunnel{
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

			By("Getting the service")
			var svc corev1.Service
			svcKey := types.NamespacedName{
				Name:      tunnel.Name,
				Namespace: "default",
			}
			Eventually(func() error { return k8sClient.Get(ctx, svcKey, &svc) }).Should(Succeed())
			Expect(svc.Spec.Type).Should(Equal(corev1.ServiceTypeClusterIP))

			By("Verifying the status")
			tunnelKey := types.NamespacedName{
				Name:      tunnel.Name,
				Namespace: tunnel.Namespace,
			}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, tunnelKey, &tunnel)).Should(Succeed())
				g.Expect(tunnel.Status.Ready).Should(BeTrue())
			}).Should(Succeed())

			By("Updating the tunnel")
			tunnelPatch := client.MergeFrom(tunnel.DeepCopy())
			tunnel.Spec.Proxy.Name = "dummy-should-not-exist"
			Expect(k8sClient.Patch(ctx, &tunnel, tunnelPatch)).Should(Succeed())

			By("Verifying the status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, tunnelKey, &tunnel)).Should(Succeed())
				g.Expect(tunnel.Status.Ready).Should(BeFalse())
			}).Should(Succeed())

			By("Verifying the service does not exist")
			err := k8sClient.Get(ctx, svcKey, &svc)
			Expect(errors.IsNotFound(err)).Should(BeTrue())
		}, SpecTimeout(3*time.Second))
	})
})
