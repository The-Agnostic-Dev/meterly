package meters

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/palmgrove-estates/meterly/internal/energy"
)

// Vendor issues STS tokens for a meter.
type Vendor interface {
	Issue(ctx context.Context, serial string, units energy.KWh) (string, error)
}

// DemoVendor issues 20-digit tokens locally instead of calling the DisCo
// vending gateway. Delay stands in for the gateway round trip so the demo
// behaves like production (the gateway usually answers in under a second).
type DemoVendor struct {
	Delay time.Duration

	mu  sync.Mutex
	rng *rand.Rand
}

// NewDemoVendor returns a vendor with a fixed random source.
func NewDemoVendor(delay time.Duration) *DemoVendor {
	return &DemoVendor{Delay: delay, rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 7))}
}

// Issue returns a new token.
func (v *DemoVendor) Issue(ctx context.Context, serial string, units energy.KWh) (string, error) {
	if serial == "" || units <= 0 {
		return "", fmt.Errorf("vend: invalid request for meter %q", serial)
	}
	if v.Delay > 0 {
		select {
		case <-time.After(v.Delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return FormatToken(v.rng), nil
}

// FormatToken builds a 20-digit token grouped in fours.
func FormatToken(rng *rand.Rand) string {
	groups := make([]string, 5)
	for i := range groups {
		groups[i] = fmt.Sprintf("%04d", rng.IntN(10000))
	}
	return strings.Join(groups, " ")
}
