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

	"github.com/int128/ktunnels/pkg/envoy"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
)

// TunnelReconciler reconciles a Tunnel object
type TunnelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=tunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=tunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=tunnels/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	var tunnel ktunnelsv1.Tunnel
	if err := r.Get(ctx, req.NamespacedName, &tunnel); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !tunnel.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	proxyKey := types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Spec.Proxy.Name}
	svcKey := types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Name}
	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, proxyKey, &proxy); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch the proxy", "proxy", proxyKey)
			return ctrl.Result{}, err
		}

		log.Error(err, "no such proxy", "proxy", proxyKey)
		tunnelPatch := client.MergeFrom(tunnel.DeepCopy())
		tunnel.Status.Ready = false
		tunnel.Status.Reason = ktunnelsv1.TunnelStatusReasonNoSuchProxy
		if err := r.Status().Patch(ctx, &tunnel, tunnelPatch); err != nil {
			log.Error(err, "unable to update the tunnel status")
			return ctrl.Result{}, err
		}
		if err := r.deleteServiceIfExists(ctx, svcKey); err != nil {
			log.Error(err, "unable to delete the service")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	if tunnel.Status.TransitPort == nil {
		tunnelPatch := client.MergeFrom(tunnel.DeepCopy())
		tunnel.Status.Ready = false
		tunnel.Status.Reason = ktunnelsv1.TunnelStatusReasonWaitForProxy
		if err := r.Status().Patch(ctx, &tunnel, tunnelPatch); err != nil {
			log.Error(err, "unable to update the tunnel status")
			return ctrl.Result{}, err
		}
		if err := r.deleteServiceIfExists(ctx, svcKey); err != nil {
			log.Error(err, "unable to delete the service")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.reconcileService(ctx, svcKey, tunnel); err != nil {
		tunnelPatch := client.MergeFrom(tunnel.DeepCopy())
		tunnel.Status.Ready = false
		tunnel.Status.Reason = ktunnelsv1.TunnelStatusReasonServiceFailure
		if err := r.Status().Patch(ctx, &tunnel, tunnelPatch); err != nil {
			log.Error(err, "unable to update the tunnel status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	tunnelPatch := client.MergeFrom(tunnel.DeepCopy())
	tunnel.Status.Ready = true
	tunnel.Status.Reason = ""
	if err := r.Status().Patch(ctx, &tunnel, tunnelPatch); err != nil {
		log.Error(err, "unable to update the tunnel status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) reconcileService(ctx context.Context, svcKey types.NamespacedName, tunnel ktunnelsv1.Tunnel) error {
	log := crlog.FromContext(ctx, "service", svcKey)

	var svc corev1.Service
	if err := r.Get(ctx, svcKey, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			svc := envoy.NewService(svcKey, tunnel)
			if err := ctrl.SetControllerReference(&tunnel, &svc, r.Scheme); err != nil {
				log.Error(err, "unable to set a controller reference to service")
				return err
			}
			if err := r.Create(ctx, &svc); err != nil {
				log.Error(err, "unable to create a service")
				return err
			}
			log.Info("created a service")
			return nil
		}

		log.Error(err, "unable to fetch the service")
		return err
	}

	svcTemplate := envoy.NewService(svcKey, tunnel)
	svcPatch := client.MergeFrom(svc.DeepCopy())
	svc.Spec.Ports = svcTemplate.Spec.Ports
	svc.Spec.Selector = svcTemplate.Spec.Selector
	if err := ctrl.SetControllerReference(&tunnel, &svc, r.Scheme); err != nil {
		log.Error(err, "unable to set a controller reference")
		return err
	}
	if err := r.Patch(ctx, &svc, svcPatch); err != nil {
		log.Error(err, "unable to update the service")
		return err
	}
	log.Info("updated the service")
	return nil
}

func (r *TunnelReconciler) deleteServiceIfExists(ctx context.Context, svcKey types.NamespacedName) error {
	log := crlog.FromContext(ctx, "service", svcKey)
	var svc corev1.Service
	if err := r.Get(ctx, svcKey, &svc); err != nil {
		return client.IgnoreNotFound(err)
	}
	if err := r.Delete(ctx, &svc); err != nil {
		return client.IgnoreNotFound(err)
	}
	log.Info("deleted the service")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Tunnel{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
