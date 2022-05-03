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

package controllers

import (
	"context"
	"fmt"
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"math/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

// TunnelReconciler reconciles a Tunnel object
type TunnelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=tunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=tunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=tunnels/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := klog.FromContext(ctx)

	var tunnel ktunnelsv1.Tunnel
	if err := r.Get(ctx, req.NamespacedName, &tunnel); err != nil {
		log.Error(err, "unable to get the tunnel")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !tunnel.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	if err := r.reconcileService(ctx, tunnel); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) reconcileService(ctx context.Context, tunnel ktunnelsv1.Tunnel) error {
	proxyKey := types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Spec.ProxyNameRef}
	serviceKey := types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Name}
	log := klog.FromContext(ctx, "service", serviceKey.Name)

	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, proxyKey, &proxy); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "unable to get the proxy for tunnel", "proxy", proxyKey.Name)
			return err
		}
		log.Error(err, "invalid proxyNameRef", "proxy", proxyKey.Name)
		return r.deleteService(ctx, serviceKey)
	}

	transitPort, err := r.allocateTransitPort(ctx, tunnel, proxy)
	if err != nil {
		return err
	}

	servicePorts := []corev1.ServicePort{{
		Name:       "envoy",
		Port:       tunnel.Spec.Port,
		TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: transitPort},
	}}
	serviceSelector := map[string]string{"ktunnels.int128.github.io/envoy": proxy.Name}

	var svc corev1.Service
	if err := r.Get(ctx, serviceKey, &svc); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "unable to get the service", "service", serviceKey)
			return err
		}

		svc.Namespace = serviceKey.Namespace
		svc.Name = serviceKey.Name
		svc.Spec.Ports = servicePorts
		svc.Spec.Selector = serviceSelector
		if err := ctrl.SetControllerReference(&tunnel, &svc, r.Scheme); err != nil {
			log.Error(err, "unable to set a controller reference to service", "service", serviceKey)
			return err
		}
		if err := r.Create(ctx, &svc); err != nil {
			log.Error(err, "unable to create a service", "service", serviceKey)
			return err
		}
		log.Info("created service", "service", serviceKey)
		return nil
	}

	svc.Spec.Ports = servicePorts
	svc.Spec.Selector = serviceSelector
	if err := ctrl.SetControllerReference(&tunnel, &svc, r.Scheme); err != nil {
		log.Error(err, "unable to set a controller reference", "service", serviceKey)
		return err
	}
	if err := r.Update(ctx, &svc); err != nil {
		log.Error(err, "unable to update the service", "service", serviceKey)
		return err
	}
	log.Info("updated service", "service", serviceKey)
	return nil
}

func (r *TunnelReconciler) allocateTransitPort(ctx context.Context, tunnel ktunnelsv1.Tunnel, proxy ktunnelsv1.Proxy) (int32, error) {
	log := klog.FromContext(ctx, "proxy", proxy.Name)

	for _, a := range proxy.Spec.TransitPortAllocation {
		if a.TunnelNameRef == tunnel.Name {
			return a.TransitPort, nil
		}
	}

	port, err := findAvailablePort(proxy)
	if err != nil {
		return 0, err
	}
	proxy.Spec.TransitPortAllocation = append(proxy.Spec.TransitPortAllocation,
		&ktunnelsv1.ProxyTransitPortAllocation{
			TunnelNameRef: tunnel.Name,
			TransitPort:   port,
		})

	if err := r.Update(ctx, &proxy); err != nil {
		log.Error(err, "unable to update the proxy to allocate a transit port")
		return 0, err
	}
	log.Info("updated the proxy with an allocated transit port")
	return port, nil
}

func findAvailablePort(proxy ktunnelsv1.Proxy) (int32, error) {
	portMap := make(map[int32]string)
	for _, a := range proxy.Spec.TransitPortAllocation {
		portMap[a.TransitPort] = a.TunnelNameRef
	}
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < 20000; i++ {
		p := int32(10000 + r.Intn(20000))
		if _, exists := portMap[p]; !exists {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available port")
}

func (r *TunnelReconciler) deleteService(ctx context.Context, key types.NamespacedName) error {
	log := klog.FromContext(ctx, "service", key.Name)
	var svc corev1.Service
	if err := r.Get(ctx, key, &svc); err != nil {
		log.Error(err, "unable to get the service")
		return client.IgnoreNotFound(err)
	}
	if err := r.Delete(ctx, &svc); err != nil {
		log.Error(err, "unable to delete the service")
		return client.IgnoreNotFound(err)
	}
	log.Info("deleted the service")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Tunnel{}).
		Complete(r)
}
