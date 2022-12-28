package transit

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func Test_allocatePort(t *testing.T) {
	t.Run("nil is given", func(t *testing.T) {
		g := AllocatePort(nil)
		if g != nil {
			t.Errorf("AllocatePort wants nil but was %v", g)
		}
	})
	t.Run("empty is given", func(t *testing.T) {
		g := AllocatePort([]*ktunnelsv1.Tunnel{})
		if g != nil {
			t.Errorf("AllocatePort wants nil but was %v", g)
		}
	})

	t.Run("one is already allocated", func(t *testing.T) {
		g := AllocatePort([]*ktunnelsv1.Tunnel{
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo",
					Port:        100,
					Proxy:       corev1.LocalObjectReference{Name: "bar"},
					TransitPort: pointer.Int32(200),
				},
			},
		})
		if g != nil {
			t.Errorf("AllocatePort wants nil but was %v", g)
		}
	})
	t.Run("all are already allocated", func(t *testing.T) {
		g := AllocatePort([]*ktunnelsv1.Tunnel{
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo1",
					Port:        100,
					Proxy:       corev1.LocalObjectReference{Name: "bar1"},
					TransitPort: pointer.Int32(1000),
				},
			},
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo2",
					Port:        200,
					Proxy:       corev1.LocalObjectReference{Name: "bar2"},
					TransitPort: pointer.Int32(2000),
				},
			},
		})
		if g != nil {
			t.Errorf("AllocatePort wants nil but was %v", g)
		}
	})

	t.Run("allocate first one", func(t *testing.T) {
		mockIntn := func(int) int { return 12345 }
		g := allocatePort([]*ktunnelsv1.Tunnel{
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:  "foo1",
					Port:  100,
					Proxy: corev1.LocalObjectReference{Name: "bar1"},
				},
			},
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo2",
					Port:        200,
					Proxy:       corev1.LocalObjectReference{Name: "bar2"},
					TransitPort: pointer.Int32(2000),
				},
			},
		}, mockIntn)
		w := []*ktunnelsv1.Tunnel{
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo1",
					Port:        100,
					Proxy:       corev1.LocalObjectReference{Name: "bar1"},
					TransitPort: pointer.Int32(22345),
				},
			},
		}
		if diff := cmp.Diff(w, g); diff != "" {
			t.Errorf("AllocatePort want != got:\n%s", diff)
		}
	})

	t.Run("allocate last one", func(t *testing.T) {
		mockIntn := func(int) int { return 12345 }
		g := allocatePort([]*ktunnelsv1.Tunnel{
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo1",
					Port:        100,
					Proxy:       corev1.LocalObjectReference{Name: "bar1"},
					TransitPort: pointer.Int32(1000),
				},
			},
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:  "foo2",
					Port:  200,
					Proxy: corev1.LocalObjectReference{Name: "bar2"},
				},
			},
		}, mockIntn)
		w := []*ktunnelsv1.Tunnel{
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo2",
					Port:        200,
					Proxy:       corev1.LocalObjectReference{Name: "bar2"},
					TransitPort: pointer.Int32(22345),
				},
			},
		}
		if diff := cmp.Diff(w, g); diff != "" {
			t.Errorf("AllocatePort want != got:\n%s", diff)
		}
	})

	t.Run("allocate all", func(t *testing.T) {
		mockIntnCounter := 12345
		mockIntn := func(int) int { mockIntnCounter++; return mockIntnCounter }

		g := allocatePort([]*ktunnelsv1.Tunnel{
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:  "foo1",
					Port:  100,
					Proxy: corev1.LocalObjectReference{Name: "bar1"},
				},
			},
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:  "foo2",
					Port:  200,
					Proxy: corev1.LocalObjectReference{Name: "bar2"},
				},
			},
		}, mockIntn)
		w := []*ktunnelsv1.Tunnel{
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo1",
					Port:        100,
					Proxy:       corev1.LocalObjectReference{Name: "bar1"},
					TransitPort: pointer.Int32(22346),
				},
			},
			{
				Spec: ktunnelsv1.TunnelSpec{
					Host:        "foo2",
					Port:        200,
					Proxy:       corev1.LocalObjectReference{Name: "bar2"},
					TransitPort: pointer.Int32(22347),
				},
			},
		}
		if diff := cmp.Diff(w, g); diff != "" {
			t.Errorf("AllocatePort want != got:\n%s", diff)
		}
	})
}
