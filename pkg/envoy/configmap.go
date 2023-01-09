package envoy

import (
	"fmt"
	"time"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	http_connection_managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tcp_proxyv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	adminClusterName  = "internal-envoy-admin"
	adminListenerName = "internal-envoy-admin"
)

func NewConfigMap(key types.NamespacedName, tunnels []*ktunnelsv1.Tunnel) (corev1.ConfigMap, error) {
	bootstrap, err := generateBootstrap()
	if err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("unable to generate bootstrap: %w", err)
	}
	cds, err := generateCDS(tunnels)
	if err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("unable to generate CDS: %w", err)
	}
	lds, err := generateLDS(tunnels)
	if err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("unable to generate LDS: %w", err)
	}
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Data: map[string]string{
			"bootstrap.json": bootstrap,
			"cds.json":       cds,
			"lds.json":       lds,
		},
	}, nil
}

func generateBootstrap() (string, error) {
	bootstrap := &bootstrapv3.Bootstrap{
		Node: &corev3.Node{
			Cluster: "test-cluster",
			Id:      "test-id",
		},
		Admin: &bootstrapv3.Admin{
			Address: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{
						Address: "127.0.0.1",
						PortSpecifier: &corev3.SocketAddress_PortValue{
							PortValue: 19901,
						},
					},
				},
			},
		},
		DynamicResources: &bootstrapv3.Bootstrap_DynamicResources{
			CdsConfig: &corev3.ConfigSource{
				ResourceApiVersion: corev3.ApiVersion_V3,
				ConfigSourceSpecifier: &corev3.ConfigSource_PathConfigSource{
					PathConfigSource: &corev3.PathConfigSource{
						Path: "/etc/envoy/cds.json",
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
						Path: "/etc/envoy/lds.json",
						WatchedDirectory: &corev3.WatchedDirectory{
							Path: "/etc/envoy",
						},
					},
				},
			},
		},
	}
	b, err := protojson.Marshal(bootstrap)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	return string(b), nil
}

func generateCDS(tunnels []*ktunnelsv1.Tunnel) (string, error) {
	var resources []*anypb.Any
	for _, tunnel := range tunnels {
		cluster := &clusterv3.Cluster{
			Name:           tunnel.Name,
			ConnectTimeout: durationpb.New(30 * time.Second),
			ClusterDiscoveryType: &clusterv3.Cluster_Type{
				Type: clusterv3.Cluster_LOGICAL_DNS,
			},
			DnsLookupFamily: clusterv3.Cluster_V4_ONLY,
			LoadAssignment: &endpointv3.ClusterLoadAssignment{
				ClusterName: tunnel.Name,
				Endpoints: []*endpointv3.LocalityLbEndpoints{
					{
						LbEndpoints: []*endpointv3.LbEndpoint{
							{
								HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
									Endpoint: &endpointv3.Endpoint{
										Address: &corev3.Address{
											Address: &corev3.Address_SocketAddress{
												SocketAddress: &corev3.SocketAddress{
													Address: tunnel.Spec.Host,
													PortSpecifier: &corev3.SocketAddress_PortValue{
														PortValue: uint32(tunnel.Spec.Port),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		r, err := anypb.New(cluster)
		if err != nil {
			return "", fmt.Errorf("anypb.New(clusterv3.Cluster): %w", err)
		}
		resources = append(resources, r)
	}

	adminCluster, err := createAdminCluster()
	if err != nil {
		return "", fmt.Errorf("unable to create an admin cluster: %w", err)
	}
	resources = append(resources, adminCluster)

	b, err := protojson.Marshal(&discoveryv3.DiscoveryResponse{Resources: resources})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	return string(b), nil
}

func createAdminCluster() (*anypb.Any, error) {
	admin, err := anypb.New(&clusterv3.Cluster{
		Name:           adminClusterName,
		ConnectTimeout: durationpb.New(30 * time.Second),
		ClusterDiscoveryType: &clusterv3.Cluster_Type{
			Type: clusterv3.Cluster_LOGICAL_DNS,
		},
		DnsLookupFamily: clusterv3.Cluster_V4_ONLY,
		LoadAssignment: &endpointv3.ClusterLoadAssignment{
			ClusterName: adminClusterName,
			Endpoints: []*endpointv3.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpointv3.LbEndpoint{
						{
							HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
								Endpoint: &endpointv3.Endpoint{
									Address: &corev3.Address{
										Address: &corev3.Address_SocketAddress{
											SocketAddress: &corev3.SocketAddress{
												Address: "127.0.0.1",
												PortSpecifier: &corev3.SocketAddress_PortValue{
													PortValue: 19901,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anypb.New(clusterv3.Cluster): %w", err)
	}
	return admin, nil
}

func generateLDS(tunnels []*ktunnelsv1.Tunnel) (string, error) {
	var resources []*anypb.Any
	for _, tunnel := range tunnels {
		if tunnel.Status.TransitPort == nil {
			continue
		}

		tcpProxyConfig, err := anypb.New(&tcp_proxyv3.TcpProxy{
			StatPrefix:       "destination",
			ClusterSpecifier: &tcp_proxyv3.TcpProxy_Cluster{Cluster: tunnel.Name},
		})
		if err != nil {
			return "", fmt.Errorf("anypb.New(tcp_proxyv3.TcpProxy): %w", err)
		}
		listener := &listenerv3.Listener{
			Name: tunnel.Name,
			Address: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{
						Address: "0.0.0.0",
						PortSpecifier: &corev3.SocketAddress_PortValue{
							PortValue: uint32(*tunnel.Status.TransitPort),
						},
					},
				},
			},
			FilterChains: []*listenerv3.FilterChain{
				{
					Filters: []*listenerv3.Filter{
						{
							Name:       "envoy.filters.network.tcp_proxy",
							ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: tcpProxyConfig},
						},
					},
				},
			},
		}
		r, err := anypb.New(listener)
		if err != nil {
			return "", fmt.Errorf("anypb.New(listenerv3.Listener): %w", err)
		}
		resources = append(resources, r)
	}

	adminListener, err := createAdminListener()
	if err != nil {
		return "", fmt.Errorf("unable to create an admin listener: %w", err)
	}
	resources = append(resources, adminListener)

	b, err := protojson.Marshal(&discoveryv3.DiscoveryResponse{Resources: resources})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	return string(b), nil
}

func createAdminListener() (*anypb.Any, error) {
	router, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("anypb.New(routerv3.Router): %w", err)
	}

	manager, err := anypb.New(&http_connection_managerv3.HttpConnectionManager{
		StatPrefix: "envoy_admin",
		HttpFilters: []*http_connection_managerv3.HttpFilter{
			{
				Name:       "envoy.filters.http.router",
				ConfigType: &http_connection_managerv3.HttpFilter_TypedConfig{TypedConfig: router},
			},
		},
		RouteSpecifier: &http_connection_managerv3.HttpConnectionManager_RouteConfig{
			RouteConfig: &routev3.RouteConfiguration{
				Name: "local_route",
				VirtualHosts: []*routev3.VirtualHost{
					{
						Name:    "local_service",
						Domains: []string{"*"},
						Routes: []*routev3.Route{
							{
								Match: &routev3.RouteMatch{
									PathSpecifier: &routev3.RouteMatch_Path{
										Path: "/ready",
									},
								},
								Action: &routev3.Route_Route{
									Route: &routev3.RouteAction{
										ClusterSpecifier: &routev3.RouteAction_Cluster{
											Cluster: adminClusterName,
										},
									},
								},
							},
							{
								Match: &routev3.RouteMatch{
									PathSpecifier: &routev3.RouteMatch_Path{
										Path: "/stats/prometheus",
									},
								},
								Action: &routev3.Route_Route{
									Route: &routev3.RouteAction{
										ClusterSpecifier: &routev3.RouteAction_Cluster{
											Cluster: adminClusterName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anypb.New(http_connection_managerv3.HttpConnectionManager): %w", err)
	}

	admin, err := anypb.New(&listenerv3.Listener{
		Name: adminListenerName,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: 9901,
					},
				},
			},
		},
		FilterChains: []*listenerv3.FilterChain{
			{
				Filters: []*listenerv3.Filter{
					{
						Name:       "envoy.filters.network.http_connection_manager",
						ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: manager},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anypb.New(listenerv3.Listener): %w", err)
	}
	return admin, nil
}
