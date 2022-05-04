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

	proxyKey := types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Spec.ProxyNameRef}
	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, proxyKey, &proxy); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "unable to get the proxy for tunnel", "proxy", proxyKey.Name)
			return ctrl.Result{}, err
		}
		log.Error(err, "invalid proxyNameRef", "proxy", proxyKey.Name)
		serviceKey := types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Name}
		return ctrl.Result{}, r.deleteService(ctx, serviceKey)
	}

	if err := r.reconcileTunnel(ctx, tunnel, proxy); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, tunnel); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) reconcileTunnel(ctx context.Context, tunnel ktunnelsv1.Tunnel, proxy ktunnelsv1.Proxy) error {
	log := klog.FromContext(ctx)

	if tunnel.Spec.TransitPort != nil {
		log.Info("transit port is already allocated; nothing to reconcile the tunnel")
		return nil
	}

	var tunnelList ktunnelsv1.TunnelList
	if err := r.List(ctx, &tunnelList,
		client.InNamespace(proxy.Namespace),
		client.MatchingFields{proxyNameRefKey: proxy.Name},
	); err != nil {
		log.Error(err, "unable to list tunnels", "proxy", proxy.Name)
		return err
	}
	log.Info("listed referenced tunnels", "count", len(tunnelList.Items))

	transitPort, err := findAvailableTransitPort(tunnelList)
	if err != nil {
		log.Error(err, "unable to find a transit port")
		return err
	}

	proxy.Spec.LastAllocatedTransitPort = &transitPort
	if err := r.Update(ctx, &proxy); err != nil {
		log.Error(err, "unable to acquire a lock to update the tunnel")
		return err
	}

	tunnel.Spec.TransitPort = &transitPort
	if err := r.Update(ctx, &tunnel); err != nil {
		log.Error(err, "unable to update the tunnel")
		return err
	}
	return nil
}

func (r *TunnelReconciler) reconcileService(ctx context.Context, tunnel ktunnelsv1.Tunnel) error {
	serviceKey := types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Name}
	log := klog.FromContext(ctx, "service", serviceKey.Name)

	if tunnel.Spec.TransitPort == nil {
		serviceKey := types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Name}
		return r.deleteService(ctx, serviceKey)
	}

	servicePorts := []corev1.ServicePort{{
		Name:       "envoy",
		Port:       tunnel.Spec.Port,
		TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: *tunnel.Spec.TransitPort},
	}}
	serviceSelector := map[string]string{"ktunnels.int128.github.io/envoy": tunnel.Spec.ProxyNameRef}

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

func findAvailableTransitPort(tunnelList ktunnelsv1.TunnelList) (int32, error) {
	portMap := make(map[int32]string)
	for _, item := range tunnelList.Items {
		if item.Spec.TransitPort == nil {
			continue
		}
		portMap[*item.Spec.TransitPort] = item.Name
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
