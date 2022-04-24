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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/apimachinery/pkg/runtime"
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

//+kubebuilder:rbac:groups=,resources=services,verbs=get;create;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	var tunnel ktunnelsv1.Tunnel
	if err := r.Get(ctx, req.NamespacedName, &tunnel); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var tunnelService corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &tunnelService); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, err
		}
	}

	tunnelServiceSpec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Protocol:   corev1.ProtocolTCP,
				Port:       tunnel.Spec.TargetPort,
				TargetPort: intstr.FromInt(0), //TODO
			},
		},
		Selector: map[string]string{
			"ktunnels.int128.github.io/proxy": tunnel.Name,
		},
	}

	if tunnelService.Name == "" {
		log.Info("creating a service for tunnel")
		tunnelService = corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: req.Namespace,
				Name:      req.Name,
			},
			Spec: tunnelServiceSpec,
		}
		if err := r.Create(ctx, &tunnelService); err != nil {
			log.Error(err, "unable to create a service for tunnel")
			return ctrl.Result{}, err
		}
		if err := ctrl.SetControllerReference(&tunnel, &tunnelService, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	tunnelService.Spec = tunnelServiceSpec
	if err := r.Update(ctx, &tunnelService); err != nil {
		log.Error(err, "unable to update the service")
		return ctrl.Result{}, err
	}
	if err := ctrl.SetControllerReference(&tunnel, &tunnelService, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Tunnel{}).
		Complete(r)
}
