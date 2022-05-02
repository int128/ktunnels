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
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
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
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if err := r.reconcileControllerReference(ctx, tunnel); err != nil {
		log.Error(err, "Unable to reconcile the controller reference")
		return ctrl.Result{}, err
	}
	if tunnel.Spec.TransitPort == nil {
		log.Info("Transit port is not yet allocated")
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) reconcileControllerReference(ctx context.Context, tunnel ktunnelsv1.Tunnel) error {
	log := klog.FromContext(ctx)

	if metav1.GetControllerOf(&tunnel) != nil {
		return nil
	}

	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Spec.ProxyNameRef}, &proxy); err != nil {
		return client.IgnoreNotFound(err)
	}
	log.Info("Updating controller reference", "tunnel", tunnel.Name, "proxy", proxy.Name)
	if err := ctrl.SetControllerReference(&proxy, &tunnel, r.Scheme); err != nil {
		return err
	}
	if err := r.Update(ctx, &tunnel); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Tunnel{}).
		Complete(r)
}
