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
	"math/rand"
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
		log.Error(err, "unable to list tunnels")
		return ctrl.Result{}, err
	}

	if err := r.reconcileTunnels(ctx, proxy, tunnelList); err != nil {
		log.Error(err, "unable to reconcile tunnels")
		return ctrl.Result{}, err
	}

	if err := r.reconcileEnvoyConfigMap(ctx, proxy, tunnelList); err != nil {
		log.Error(err, "unable to reconcile envoy ConfigMap")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ProxyReconciler) reconcileTunnels(ctx context.Context, proxy ktunnelsv1.Proxy, tunnelList ktunnelsv1.TunnelList) error {
	log := klog.FromContext(ctx)

	var portMap = make(map[int32]string)
	var needToReconcile []ktunnelsv1.Tunnel
	for _, tunnel := range tunnelList.Items {
		tunnel := tunnel
		if tunnel.Spec.TransitPort == nil {
			needToReconcile = append(needToReconcile, tunnel)
			continue
		}
		p := *tunnel.Spec.TransitPort
		if _, exists := portMap[p]; exists {
			// transit port is duplicated
			needToReconcile = append(needToReconcile, tunnel)
			continue
		}
		portMap[p] = tunnel.Name
	}

	for _, tunnel := range needToReconcile {
		tunnel := tunnel
		p, err := findPort(portMap, rand.NewSource(proxy.Generation))
		if err != nil {
			return fmt.Errorf("unable to allocate transit port for tunnel %s: %w", tunnel.Name, err)
		}
		log.Info("Updating transit port of tunnel", "tunnel", tunnel.Name, "transitPort", p)
		tunnel.Spec.TransitPort = &p
		if err := r.Update(ctx, &tunnel); err != nil {
			return fmt.Errorf("unable to update tunnel %s: %w", tunnel.Name, err)
		}
		portMap[p] = tunnel.Name
	}
	return nil
}

func findPort(portMap map[int32]string, src rand.Source) (int32, error) {
	r := rand.New(src)
	for i := 0; i < 20000; i++ {
		p := int32(10000 + r.Intn(20000))
		if _, exists := portMap[p]; !exists {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available port")
}

func (r *ProxyReconciler) reconcileEnvoyConfigMap(ctx context.Context, proxy ktunnelsv1.Proxy, tunnelList ktunnelsv1.TunnelList) error {
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

		log.Info("Creating envoy ConfigMap", "configMap", cmName)
		cm.Namespace = cmName.Namespace
		cm.Name = cmName.Name
		cm.Data = generateEnvoyConfigMapData(tunnelList)
		if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
			return fmt.Errorf("unable to set a controller reference: %w", err)
		}
		if err := r.Create(ctx, &cm); err != nil {
			return fmt.Errorf("unable to create the proxy: %w", err)
		}
		return nil
	}

	log.Info("Updating envoy ConfigMap", "configMap", cmName)
	cm.Data = generateEnvoyConfigMapData(tunnelList)
	if err := ctrl.SetControllerReference(&proxy, &cm, r.Scheme); err != nil {
		return fmt.Errorf("unable to set a controller reference: %w", err)
	}
	if err := r.Update(ctx, &cm); err != nil {
		return fmt.Errorf("unable to update the proxy: %w", err)
	}
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
		t := item.Spec
		if t.TransitPort == nil {
			continue
		}
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
`, *t.TransitPort, *t.TransitPort, t.Host, t.Port))
	}
	return sb.String()
}

func generateLDS(tunnelList ktunnelsv1.TunnelList) string {
	var sb strings.Builder
	sb.WriteString(`# lds.yaml
resources:
`)
	for _, item := range tunnelList.Items {
		t := item.Spec
		if t.TransitPort == nil {
			continue
		}
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
`, *t.TransitPort, *t.TransitPort, *t.TransitPort))
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
		Owns(&ktunnelsv1.Tunnel{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
