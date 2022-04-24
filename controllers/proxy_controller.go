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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
)

// ProxyReconciler reconciles a Proxy object
type ProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ktunnels.int128.github.io,resources=proxies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, req.NamespacedName, &proxy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO: add prefix to name
	var proxyDeployment appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &proxyDeployment); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, err
		}
	}

	proxyDeploymentSpec := appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(1),
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "envoy",
					Image: "envoyproxy/envoy:v1.23-latest",
					Args:  []string{"--config-yaml", ``},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("10Mi"),
						},
					},
				}},
			},
		},
	}

	if proxyDeployment.Name == "" {
		proxyDeployment = appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: req.Namespace,
				Name:      req.Name,
			},
			Spec: proxyDeploymentSpec,
		}
		if err := r.Create(ctx, &proxyDeployment); err != nil {
			log.Error(err, "unable to create a proxy deployment")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	proxyDeployment.Spec = proxyDeploymentSpec
	if err := r.Update(ctx, &proxyDeployment); err != nil {
		log.Error(err, "unable to update the proxy deployment")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Proxy{}).
		Complete(r)
}
