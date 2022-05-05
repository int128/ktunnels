package envoy

import (
	"fmt"
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"strconv"
	"strings"
)

func NewConfigMap(key types.NamespacedName, tunnels []*ktunnelsv1.Tunnel) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Data: generateConfigMapData(tunnels),
	}
}

func generateConfigMapData(tunnels []*ktunnelsv1.Tunnel) map[string]string {
	return map[string]string{
		"bootstrap.yaml": generateBootstrap(),
		"cds.yaml":       generateCDS(tunnels),
		"lds.yaml":       generateLDS(tunnels),
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

func generateCDS(tunnels []*ktunnelsv1.Tunnel) string {
	var sb strings.Builder
	sb.WriteString(`# cds.yaml
resources:
`)
	for _, item := range tunnels {
		if item.Spec.TransitPort == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf(`
  - "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
    name: %s
    connect_timeout: 30s
    type: LOGICAL_DNS
    dns_lookup_family: V4_ONLY
    load_assignment:
      cluster_name: %s
      endpoints:
        - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: %s
                    port_value: %d
`,
			strconv.Quote(item.Name),
			strconv.Quote(item.Name),
			strconv.Quote(item.Spec.Host),
			item.Spec.Port,
		))
	}
	return sb.String()
}

func generateLDS(tunnels []*ktunnelsv1.Tunnel) string {
	var sb strings.Builder
	sb.WriteString(`# lds.yaml
resources:
`)
	for _, item := range tunnels {
		if item.Spec.TransitPort == nil {
			continue
		}
		transitPort := *item.Spec.TransitPort

		sb.WriteString(fmt.Sprintf(`
  - "@type": type.googleapis.com/envoy.config.listener.v3.Listener
    name: %s
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
              cluster: %s
`,
			strconv.Quote(item.Name),
			transitPort,
			strconv.Quote(item.Name),
		))
	}
	return sb.String()
}
