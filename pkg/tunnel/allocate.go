package tunnel

import (
	"errors"
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
)

type portSet map[int32]interface{}

func FillTransitPort(tunnels []*ktunnelsv1.ProxyTunnel) (bool, error) {
	assigned := make(portSet)
	var unassigned []*ktunnelsv1.ProxyTunnel
	for _, t := range tunnels {
		if t.TransitPort > 0 {
			_, exists := assigned[t.TransitPort]
			if !exists {
				assigned[t.TransitPort] = 1
				continue
			}
		}
		unassigned = append(unassigned, t)
	}

	if len(unassigned) == 0 {
		return false, nil
	}

	for _, t := range unassigned {
		availablePort := findAvailablePort(assigned)
		if availablePort == -1 {
			return false, errors.New("no available port")
		}
		t.TransitPort = availablePort
		assigned[availablePort] = 1
	}
	return true, nil
}

func findAvailablePort(assigned portSet) int32 {
	for p := int32(10000); p < 30000; p++ {
		if _, exists := assigned[p]; !exists {
			return p
		}
	}
	return -1
}
