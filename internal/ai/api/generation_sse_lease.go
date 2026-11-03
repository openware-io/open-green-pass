package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

var generationSSELeaseFallback atomic.Uint64

// newGenerationSSELeaseID returns a per-connection unique lease id. Random ids
// keep leases distinct across replicas; the timestamp/counter fallback is only
// used if the OS random source fails (extremely unlikely) and remains unique
// within the process.
func newGenerationSSELeaseID() string {
	var buffer [16]byte
	if _, err := rand.Read(buffer[:]); err == nil {
		return hex.EncodeToString(buffer[:])
	}
	return fmt.Sprintf("sse-%d-%d", time.Now().UnixNano(), generationSSELeaseFallback.Add(1))
}
