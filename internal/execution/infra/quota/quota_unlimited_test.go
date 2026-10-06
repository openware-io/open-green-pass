package quota

import (
	"context"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

func TestMemoryQuotaZeroLimitIsUnlimited(t *testing.T) {
	manager := NewMemoryQuota(map[string]Limits{"run": {}})
	request := domain.ResourceRequest{TeamID: 1, TargetID: 2, OwnerID: 3, Type: "run", Units: 1}
	for index := 0; index < 100; index++ {
		if err := manager.Acquire(context.Background(), request); err != nil {
			t.Fatalf("acquire %d: %v", index, err)
		}
	}
}
