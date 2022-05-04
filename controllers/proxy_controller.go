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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"math/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

const (
	proxyNameRefKey = ".spec.proxyNameRef"
)

// ProxyReconciler reconciles a Proxy object
type ProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies/finalizers,verbs=update

//+kubebuilder:rbac:groups=,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := klog.FromContext(ctx)

	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, req.NamespacedName, &proxy); err != nil {
		log.Error(err, "unable to get the proxy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !proxy.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	var tunnelList ktunnelsv1.TunnelList
	if err := r.List(ctx, &tunnelList,
		client.InNamespace(proxy.ObjectMeta.Namespace),
		client.MatchingFields{proxyNameRefKey: proxy.Name},
	); err != nil {
		log.Error(err, "unable to list tunnels", "proxy", proxy.Name)
		return ctrl.Result{}, err
	}
	log.Info("listed referenced tunnels", "count", len(tunnelList.Items))

	if err := r.reconcileTunnels(ctx, tunnelList, proxy); err != nil {
		log.Error(err, "unable to reconcile tunnels")
		return ctrl.Result{}, err
	}
	if err := r.reconcileConfigMap(ctx, proxy, tunnelList); err != nil {
		log.Error(err, "unable to reconcile config map")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ProxyReconciler) reconcileTunnels(ctx context.Context, tunnelList ktunnelsv1.TunnelList, proxy ktunnelsv1.Proxy) error {
	log := klog.FromContext(ctx)

	allocatedTunnels := allocateTransitPort(tunnelList)
	if len(allocatedTunnels) == 0 {
		log.Info("all tunnels are consistently allocated")
		return nil
	}

	for _, tunnel := range allocatedTunnels {
		if err := r.Update(ctx, tunnel); err != nil {
			log.Error(err, "unable to update the tunnel")
			return err
		}
		log.Info("updated the tunnel", "tunnel", tunnel.Name)
	}
	if err := r.Update(ctx, &proxy); err != nil {
		log.Error(err, "unable to acquire an optimistic lock to update the tunnels", "proxy", proxy.Name)
		return err
	}
	log.Info("successfully updated the tunnels consistently", "proxy", proxy.Name)
	return nil
}

func allocateTransitPort(tunnelList ktunnelsv1.TunnelList) []*ktunnelsv1.Tunnel {
	var needToReconcile []*ktunnelsv1.Tunnel
	var portMap = make(map[int32]string)
	for _, item := range tunnelList.Items {
		item := item
		// not allocated
		if item.Spec.TransitPort == nil {
			needToReconcile = append(needToReconcile, &item)
			continue
		}
		// dedupe
		if _, exists := portMap[*item.Spec.TransitPort]; exists {
			needToReconcile = append(needToReconcile, &item)
			continue
		}
		portMap[*item.Spec.TransitPort] = item.Name
	}

	for _, item := range needToReconcile {
		item.Spec.TransitPort = findPort(portMap)
	}
	return needToReconcile
}

func findPort(portMap map[int32]string) *int32 {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < 20000; i++ {
		p := int32(10000 + r.Intn(20000))
		if _, exists := portMap[p]; !exists {
			return &p
		}
	}
	return nil
}

func (r *ProxyReconciler) reconcileConfigMap(ctx context.Context, proxy ktunnelsv1.Proxy, tunnelList ktunnelsv1.TunnelList) error {
	cmKey := types.NamespacedName{
		Namespace: proxy.Namespace,
		Name:      fmt.Sprintf("%s-envoy", proxy.Name),
	}
	log := klog.FromContext(ctx, "configMap", cmKey)

	var cm corev1.ConfigMap
	if err := r.Get(ctx, cmKey, &cm); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			return fmt.Errorf("unable to get the proxy: %w", err)
		}

		cm := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cmKey.Namespace,
				Name:      cmKey.Name,
			},
			Data: generateEnvoyConfigMapData(tunnelList),
		}
		if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
			log.Error(err, "unable to set a controller reference to proxy")
			return err
		}
		if err := r.Create(ctx, &cm); err != nil {
			log.Error(err, "unable to create a config map")
			return err
		}
		log.Info("created a config map")
		return nil
	}

	cm.Data = generateEnvoyConfigMapData(tunnelList)
	if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
		log.Error(err, "unable to set a controller reference to proxy")
		return err
	}
	if err := r.Update(ctx, &cm); err != nil {
		log.Error(err, "unable to update the config map")
		return err
	}
	log.Info("updated the config map")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&ktunnelsv1.Tunnel{},
		proxyNameRefKey,
		func(rawObj client.Object) []string {
			tunnel, ok := rawObj.(*ktunnelsv1.Tunnel)
			if !ok {
				return nil
			}
			return []string{tunnel.Spec.ProxyNameRef}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Proxy{}).
		Owns(&corev1.ConfigMap{}).
		Watches(
			&source.Kind{Type: &ktunnelsv1.Tunnel{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForTunnel),
		).
		Complete(r)
}

func (r *ProxyReconciler) findObjectsForTunnel(obj client.Object) []reconcile.Request {
	tunnel, ok := obj.(*ktunnelsv1.Tunnel)
	if !ok {
		return nil
	}
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: tunnel.Namespace,
			Name:      tunnel.Spec.ProxyNameRef,
		},
	}}
}
