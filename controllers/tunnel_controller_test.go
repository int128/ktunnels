package controllers

import (
	"context"
	"time"

	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Tunnel controller", func() {
	Context("When a tunnel is created", func() {
		It("Should create a service", func(ctx context.Context) {
			By("Creating a proxy")
			proxy := ktunnelsv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "example-",
					Namespace:    "default",
				},
			}
			Expect(k8sClient.Create(ctx, &proxy)).Should(Succeed())

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

		}, SpecTimeout(3*time.Second))
	})
})
