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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"

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

//+kubebuilder:rbac:groups=,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var proxy ktunnelsv1.Proxy
	if err := r.Get(ctx, req.NamespacedName, &proxy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !proxy.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	if err := r.reconcileEnvoyConfigMap(ctx, proxy); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ProxyReconciler) reconcileEnvoyConfigMap(ctx context.Context, proxy ktunnelsv1.Proxy) error {
	log := klog.FromContext(ctx)

	cmName := types.NamespacedName{
		Namespace: proxy.ObjectMeta.Namespace,
		Name:      fmt.Sprintf("%s-envoy", proxy.ObjectMeta.Name),
	}
	var cm corev1.ConfigMap
	if err := r.Get(ctx, cmName, &cm); err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			return fmt.Errorf("unable to get the proxy: %w", err)
		}

		log.Info("Creating ConfigMap", "ConfigMap", cmName)
		cm.Namespace = cmName.Namespace
		cm.Name = cmName.Name
		cm.Data = generateEnvoyConfigMapData(proxy)
		if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
			return fmt.Errorf("unable to set a controller reference: %w", err)
		}
		if err := r.Create(ctx, &cm); err != nil {
			return fmt.Errorf("unable to create the proxy: %w", err)
		}
		return nil
	}

	log.Info("Updating ConfigMap", "ConfigMap", cmName)
	cm.Data = generateEnvoyConfigMapData(proxy)
	if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
		return fmt.Errorf("unable to set a controller reference: %w", err)
	}
	if err := r.Update(ctx, &cm); err != nil {
		return fmt.Errorf("unable to update the proxy: %w", err)
	}
	return nil
}

func generateEnvoyConfigMapData(proxy ktunnelsv1.Proxy) map[string]string {
	return map[string]string{
		"bootstrap.yaml": generateBootstrap(),
		"cds.yaml":       generateCDS(proxy),
		"lds.yaml":       generateLDS(proxy),
	}
}

func generateBootstrap() string {
	return `# bootstrap.yaml
node:
  cluster: test-cluster
  id: test-id

dynamic_resources:
  cds_config:
    resource_api_version: V3
    path_config_source:
      path: /etc/envoy/cds.yaml
      watched_directory:
        path: /etc/envoy
  lds_config:
    resource_api_version: V3
    path_config_source:
      path: /etc/envoy/lds.yaml
      watched_directory:
        path: /etc/envoy
`
}

func generateCDS(proxy ktunnelsv1.Proxy) string {
	var sb strings.Builder
	sb.WriteString(`# cds.yaml
resources:
`)
	for i, tunnel := range proxy.Spec.Tunnels {
		sb.WriteString(fmt.Sprintf(`
  - "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
    name: cluster_0
    connect_timeout: 30s
    type: LOGICAL_DNS
    dns_lookup_family: V4_ONLY
    load_assignment:
      cluster_name: cluster_%d
      endpoints:
        - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: %s
                    port_value: %d
`, i, tunnel.Host, tunnel.Port))
	}
	return sb.String()
}

func generateLDS(proxy ktunnelsv1.Proxy) string {
	var sb strings.Builder
	sb.WriteString(`# lds.yaml
resources:
`)
	for i := range proxy.Spec.Tunnels {
		sb.WriteString(fmt.Sprintf(`
  - "@type": type.googleapis.com/envoy.config.listener.v3.Listener
    name: listener_%d
    address:
      socket_address:
        address: 0.0.0.0
        port_value: %d
    filter_chains:
      - filters:
          - name: envoy.filters.network.tcp_proxy
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
              stat_prefix: destination
              cluster: cluster_%d
`, i, 10000+i, i))
	}
	return sb.String()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ktunnelsv1.Proxy{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
