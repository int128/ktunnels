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
	"github.com/int128/ktunnels/pkg/envoy"
	"github.com/int128/ktunnels/pkg/transit"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
)

const (
	proxyNameKey = ".spec.proxy.name"
)

// ProxyReconciler reconciles a Proxy object
type ProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, req.NamespacedName, &proxy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !proxy.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	var tunnelList ktunnelsv1.TunnelList
	if err := r.List(ctx, &tunnelList,
		client.InNamespace(proxy.ObjectMeta.Namespace),
		client.MatchingFields{proxyNameKey: proxy.Name},
	); err != nil {
		log.Error(err, "unable to fetch tunnels")
		return ctrl.Result{}, err
	}
	log.Info("fetched referenced tunnels", "tunnels", len(tunnelList.Items))

	mutableTunnels := make([]*ktunnelsv1.Tunnel, len(tunnelList.Items))
	for i := range tunnelList.Items {
		mutableTunnels[i] = &tunnelList.Items[i]
	}

	if err := r.reconcileTunnels(ctx, mutableTunnels); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("successfully reconciled the tunnels")

	if err := r.reconcileConfigMap(ctx, proxy, mutableTunnels); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("successfully reconciled the config map")

	if err := r.reconcileDeployment(ctx, proxy); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("successfully reconciled the deployment")

	return ctrl.Result{}, nil
}

func (r *ProxyReconciler) reconcileTunnels(ctx context.Context, mutableTunnels []*ktunnelsv1.Tunnel) error {
	log := crlog.FromContext(ctx)

	allocatedTunnels := transit.AllocatePort(mutableTunnels)
	if len(allocatedTunnels) == 0 {
		log.Info("all tunnels are already allocated")
		return nil
	}
	for _, tunnel := range allocatedTunnels {
		if err := r.Update(ctx, tunnel); err != nil {
			log.Error(err, "unable to update the tunnel", "tunnel", tunnel.Name)
			return err
		}
		log.Info("updated the tunnel", "tunnel", tunnel.Name)
	}
	return nil
}

func (r *ProxyReconciler) reconcileConfigMap(ctx context.Context, proxy ktunnelsv1.Proxy, mutableTunnels []*ktunnelsv1.Tunnel) error {
	cmKey := types.NamespacedName{Namespace: proxy.Namespace, Name: fmt.Sprintf("ktunnels-proxy-%s", proxy.Name)}
	log := crlog.FromContext(ctx, "configMap", cmKey)

	var cm corev1.ConfigMap
	if err := r.Get(ctx, cmKey, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			cm := envoy.NewConfigMap(cmKey, mutableTunnels)
			if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
				log.Error(err, "unable to set a controller reference")
				return err
			}
			if err := r.Create(ctx, &cm); err != nil {
				log.Error(err, "unable to create a config map")
				return err
			}
			log.Info("created a config map")
			return nil
		}

		log.Error(err, "unable to fetch the config map")
		return err
	}

	cmTemplate := envoy.NewConfigMap(cmKey, mutableTunnels)
	cm.Data = cmTemplate.Data
	if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
		log.Error(err, "unable to set a controller reference")
		return err
	}
	if err := r.Update(ctx, &cm); err != nil {
		log.Error(err, "unable to update the config map")
		return err
	}
	log.Info("updated the config map")
	return nil
}

func (r *ProxyReconciler) reconcileDeployment(ctx context.Context, proxy ktunnelsv1.Proxy) error {
	deploymentKey := types.NamespacedName{Namespace: proxy.Namespace, Name: fmt.Sprintf("ktunnels-proxy-%s", proxy.Name)}
	log := crlog.FromContext(ctx, "deployment", deploymentKey)

	var deployment appsv1.Deployment
	if err := r.Get(ctx, deploymentKey, &deployment); err != nil {
		if apierrors.IsNotFound(err) {
			deployment := envoy.NewDeployment(deploymentKey, proxy)
			if err := ctrl.SetControllerReference(&proxy, &deployment, r.Scheme); err != nil {
				log.Error(err, "unable to set a controller reference")
				return err
			}
			if err := r.Create(ctx, &deployment); err != nil {
				log.Error(err, "unable to create a deployment")
				return err
			}
			log.Info("created a deployment")
			return nil
		}

		log.Error(err, "unable to fetch the deployment")
		return err
	}

	deploymentTemplate := envoy.NewDeployment(deploymentKey, proxy)
	deployment.Spec = deploymentTemplate.Spec
	if err := ctrl.SetControllerReference(&proxy, &deployment, r.Scheme); err != nil {
		log.Error(err, "unable to set a controller reference")
		return err
	}
	if err := r.Update(ctx, &deployment); err != nil {
		log.Error(err, "unable to update the deployment")
		return err
	}
	log.Info("updated the deployment")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&ktunnelsv1.Tunnel{},
		proxyNameKey,
		mapTunnelToProxyName,
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Proxy{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Watches(
			// watch tunnel(s) of a proxy
			// https://book.kubebuilder.io/reference/watching-resources/externally-managed.html
			&source.Kind{Type: &ktunnelsv1.Tunnel{}},
			handler.EnqueueRequestsFromMapFunc(mapTunnelToReconcileRequest),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func mapTunnelToProxyName(obj client.Object) []string {
	tunnel, ok := obj.(*ktunnelsv1.Tunnel)
	if !ok {
		return nil
	}
	return []string{tunnel.Spec.Proxy.Name}
}

func mapTunnelToReconcileRequest(obj client.Object) []reconcile.Request {
	tunnel, ok := obj.(*ktunnelsv1.Tunnel)
	if !ok {
		return nil
	}
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: tunnel.Namespace,
			Name:      tunnel.Spec.Proxy.Name,
		},
	}}
}
