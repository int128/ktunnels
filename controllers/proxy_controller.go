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

	if err := r.reconcileConfigMap(ctx, proxy, tunnelList); err != nil {
		log.Error(err, "unable to reconcile ConfigMap for Envoy")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
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

		cm.Namespace = cmKey.Namespace
		cm.Name = cmKey.Name
		cm.Data = generateEnvoyConfigMapData(tunnelList)
		if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
			log.Error(err, "unable to set a controller reference to proxy")
			return err
		}
		if err := r.Create(ctx, &cm); err != nil {
			log.Error(err, "unable to create a ConfigMap")
			return err
		}
		log.Info("created a ConfigMap for Envoy")
		return nil
	}

	cm.Data = generateEnvoyConfigMapData(tunnelList)
	if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
		log.Error(err, "unable to set a controller reference to proxy")
		return err
	}
	if err := r.Update(ctx, &cm); err != nil {
		log.Error(err, "unable to update the ConfigMap")
		return err
	}
	log.Info("updated the ConfigMap for Envoy")
	return nil
}

func generateEnvoyConfigMapData(tunnelList ktunnelsv1.TunnelList) map[string]string {
	return map[string]string{
		"bootstrap.yaml": generateBootstrap(),
		"cds.yaml":       generateCDS(tunnelList),
		"lds.yaml":       generateLDS(tunnelList),
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

func generateCDS(tunnelList ktunnelsv1.TunnelList) string {
	var sb strings.Builder
	sb.WriteString(`# cds.yaml
resources:
`)
	for _, item := range tunnelList.Items {
		if item.Spec.TransitPort == nil {
			continue
		}
		transitPort := *item.Spec.TransitPort

		sb.WriteString(fmt.Sprintf(`
  - "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
    name: cluster_%d
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
`,
			transitPort,
			transitPort,
			item.Spec.Host,
			item.Spec.Port,
		))
	}
	return sb.String()
}

func generateLDS(tunnelList ktunnelsv1.TunnelList) string {
	var sb strings.Builder
	sb.WriteString(`# lds.yaml
resources:
`)
	for _, item := range tunnelList.Items {
		if item.Spec.TransitPort == nil {
			continue
		}
		transitPort := *item.Spec.TransitPort

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
`,
			transitPort,
			transitPort,
			transitPort,
		))
	}
	return sb.String()
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
		Complete(r)
}
