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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
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
		log.Error(err, "unable to get the tunnel")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !tunnel.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	if err := r.reconcileTunnel(ctx, tunnel); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, tunnel); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) reconcileTunnel(ctx context.Context, tunnel ktunnelsv1.Tunnel) error {
	log := klog.FromContext(ctx)

	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, types.NamespacedName{Namespace: tunnel.Namespace, Name: tunnel.Spec.ProxyNameRef}, &proxy); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "unable to get the proxy for tunnel", "proxy", tunnel.Spec.ProxyNameRef)
			return err
		}

		tunnel.OwnerReferences = nil
		tunnel.Spec.TransitPort = nil
		if err := r.Update(ctx, &tunnel); err != nil {
			log.Error(err, "unable to update the tunnel")
			return err
		}
		log.Info("updated the tunnel to clear the reference", "proxy", tunnel.Spec.ProxyNameRef)
		return nil
	}

	if metav1.GetControllerOf(&tunnel) != nil {
		return nil
	}
	if err := ctrl.SetControllerReference(&proxy, &tunnel, r.Scheme); err != nil {
		log.Error(err, "unable to set a controller reference to tunnel")
		return err
	}
	if err := r.Update(ctx, &tunnel); err != nil {
		log.Error(err, "unable to update the tunnel")
		return err
	}
	return nil
}

func (r *TunnelReconciler) reconcileService(ctx context.Context, tunnel ktunnelsv1.Tunnel) error {
	log := klog.FromContext(ctx)
	serviceName := types.NamespacedName{
		Namespace: tunnel.Namespace,
		Name:      tunnel.Name,
	}
	servicePorts := []corev1.ServicePort{{
		Name: "envoy",
		Port: tunnel.Spec.Port,
		TargetPort: intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: pointer.Int32PtrDerefOr(tunnel.Spec.TransitPort, 0),
		},
	}}

	var svc corev1.Service
	if err := r.Get(ctx, serviceName, &svc); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "unable to get the service", "service", serviceName)
			return err
		}

		if tunnel.Spec.TransitPort == nil {
			return nil
		}

		svc.Namespace = serviceName.Namespace
		svc.Name = serviceName.Name
		svc.Spec.Ports = servicePorts
		if err := ctrl.SetControllerReference(&tunnel, &svc, r.Scheme); err != nil {
			log.Error(err, "unable to set a controller reference to service", "service", serviceName)
			return err
		}
		if err := r.Create(ctx, &svc); err != nil {
			log.Error(err, "unable to create a service", "service", serviceName)
			return err
		}
		log.Info("created service", "service", serviceName)
		return nil
	}

	if tunnel.Spec.TransitPort == nil {
		if err := r.Delete(ctx, &svc); err != nil {
			log.Error(err, "unable to delete the service", "service", serviceName)
			return client.IgnoreNotFound(err)
		}
		log.Info("deleted service", "service", serviceName)
		return nil
	}

	svc.Spec.Ports = servicePorts
	if err := ctrl.SetControllerReference(&tunnel, &svc, r.Scheme); err != nil {
		log.Error(err, "unable to set a controller reference", "service", serviceName)
		return err
	}
	if err := r.Update(ctx, &svc); err != nil {
		log.Error(err, "unable to update the service", "service", serviceName)
		return err
	}
	log.Info("updated service", "service", serviceName)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Tunnel{}).
		Complete(r)
}
