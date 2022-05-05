package transit

import (
	ktunnelsv1 "github.com/int128/ktunnels/api/v1"
	"math/rand"
	"time"
)

// AllocatePort updates nil transit port(s) to available port(s).
// It returns the items which has been changed.
// Given array will be changed.
func AllocatePort(mutableTunnels []*ktunnelsv1.Tunnel) []*ktunnelsv1.Tunnel {
	var needToReconcile []*ktunnelsv1.Tunnel
	var portSet = make(map[int32]struct{})

	for _, item := range mutableTunnels {
		// tunnel is not allocated
		if item.Spec.TransitPort == nil {
			needToReconcile = append(needToReconcile, item)
			continue
		}
		// dedupe
		if _, exists := portSet[*item.Spec.TransitPort]; exists {
			needToReconcile = append(needToReconcile, item)
			continue
		}
		portSet[*item.Spec.TransitPort] = struct{}{}
	}

	r := rand.New(rand.NewSource(time.Now().UnixMicro()))
	for _, item := range needToReconcile {
		p := allocatePort(portSet, r)
		item.Spec.TransitPort = p
	}
	return needToReconcile
}

const (
	minPort       = 10000
	maxPort       = 30000
	allocationTry = maxPort - minPort
)

func allocatePort(portSet map[int32]struct{}, r *rand.Rand) *int32 {
	for i := 0; i < allocationTry; i++ {
		p := int32(minPort + r.Intn(maxPort-minPort+1))
		if _, exists := portSet[p]; !exists {
			portSet[p] = struct{}{}
			return &p
		}
	}
	// no available port
	return nil
}
