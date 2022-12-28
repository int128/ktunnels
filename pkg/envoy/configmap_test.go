package envoy

import (
	"encoding/json"
	"strings"
	"testing"

	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

type discoveryResponseType struct {
	Resources []struct {
		Type string `json:"@type"`
	} `json:"resources"`
}

func Test_generateCDS(t *testing.T) {
	cds, err := generateCDS([]*ktunnelsv1.Tunnel{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "microservice-database",
				Namespace: "default",
			},
			Spec: ktunnelsv1.TunnelSpec{
				Host:  "microservice-database.staging",
				Port:  5432,
				Proxy: corev1.LocalObjectReference{Name: "example"},
			},
		},
	})
	if err != nil {
		t.Fatalf("generateCDS: %s", err)
	}
	t.Logf("cds=%s", cds)

	var cdsValue discoveryResponseType
	if err := json.NewDecoder(strings.NewReader(cds)).Decode(&cdsValue); err != nil {
		t.Fatalf("unable to decode CDS json: %s", err)
	}
	if len(cdsValue.Resources) != 1 {
		t.Errorf("len(resources) wants 1 but got %d", len(cdsValue.Resources))
	}
	want := "type.googleapis.com/envoy.config.cluster.v3.Cluster"
	got := cdsValue.Resources[0].Type
	if want != got {
		t.Errorf("resources.@type wants %s but got %s", want, got)
	}
}

func Test_generateLDS(t *testing.T) {
	lds, err := generateLDS([]*ktunnelsv1.Tunnel{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "microservice-database",
				Namespace: "default",
			},
			Spec: ktunnelsv1.TunnelSpec{
				Host:        "microservice-database.staging",
				Port:        5432,
				Proxy:       corev1.LocalObjectReference{Name: "example"},
				TransitPort: pointer.Int32(30000),
			},
		},
	})
	if err != nil {
		t.Fatalf("generateLDS: %s", err)
	}
	t.Logf("lds=%s", lds)

	var ldsValue discoveryResponseType
	if err := json.NewDecoder(strings.NewReader(lds)).Decode(&ldsValue); err != nil {
		t.Fatalf("unable to decode LDS json: %s", err)
	}
	if len(ldsValue.Resources) != 1 {
		t.Errorf("len(resources) wants 1 but got %d", len(ldsValue.Resources))
	}
	want := "type.googleapis.com/envoy.config.listener.v3.Listener"
	got := ldsValue.Resources[0].Type
	if want != got {
		t.Errorf("resources.@type wants %s but got %s", want, got)
	}
}
