package envoy

import (
	"fmt"
	"strconv"
	"strings"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	"google.golang.org/protobuf/encoding/protojson"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewConfigMap(key types.NamespacedName, tunnels []*ktunnelsv1.Tunnel) (corev1.ConfigMap, error) {
	data, err := generateConfigMapData(tunnels)
	if err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("unable to generate an envoy config: %w", err)
	}
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Data: data,
	}, nil
}

func generateConfigMapData(tunnels []*ktunnelsv1.Tunnel) (map[string]string, error) {
	bootstrap, err := generateBootstrap()
	if err != nil {
		return nil, fmt.Errorf("unable to generate bootstrap: %w", err)
	}
	return map[string]string{
		"bootstrap.json": bootstrap,
		"cds.yaml":       generateCDS(tunnels),
		"lds.yaml":       generateLDS(tunnels),
	}, nil
}

func generateBootstrap() (string, error) {
	b, err := protojson.Marshal(&bootstrapv3.Bootstrap{
		Node: &corev3.Node{
			Cluster: "test-cluster",
			Id:      "test-id",
		},
		DynamicResources: &bootstrapv3.Bootstrap_DynamicResources{
			CdsConfig: &corev3.ConfigSource{
				ResourceApiVersion: corev3.ApiVersion_V3,
				ConfigSourceSpecifier: &corev3.ConfigSource_PathConfigSource{
					PathConfigSource: &corev3.PathConfigSource{
						Path: "/etc/envoy/cds.yaml",
						WatchedDirectory: &corev3.WatchedDirectory{
							Path: "/etc/envoy",
						},
					},
				},
			},
			LdsConfig: &corev3.ConfigSource{
				ResourceApiVersion: corev3.ApiVersion_V3,
				ConfigSourceSpecifier: &corev3.ConfigSource_PathConfigSource{
					PathConfigSource: &corev3.PathConfigSource{
						Path: "/etc/envoy/lds.yaml",
						WatchedDirectory: &corev3.WatchedDirectory{
							Path: "/etc/envoy",
						},
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	return string(b), nil
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
