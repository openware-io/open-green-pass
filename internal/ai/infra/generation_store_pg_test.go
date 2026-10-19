package infra

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/pkg/clock"
	"github.com/openware-io/open-green-pass/pkg/id"
)

func TestGenerationStoreIDsUseSharedBrowserSafeGenerator(t *testing.T) {
	gen, err := id.New(1, clock.NewMock(time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	store := NewGenerationBatchStore(nil, gen)
	// The first candidate is allocated by the workflow; subsequent candidates
	// and source snapshots must share the same sequence, never UnixNano IDs.
	firstSeed := gen.Next()
	for i := 0; i < 8; i++ {
		got := store.nextID()
		if got != firstSeed+int64(i)+1 || got > 1<<53-1 {
			t.Fatalf("candidate/snapshot ID %d is not from the shared safe sequence", got)
		}
		raw, err := json.Marshal(GenerationSeed{ID: got})
		if err != nil {
			t.Fatal(err)
		}
		var browser map[string]any
		if err := json.Unmarshal(raw, &browser); err != nil {
			t.Fatal(err)
		}
		if int64(browser["id"].(float64)) != got {
			t.Fatalf("ID %d changed during browser-style JSON decoding", got)
		}
	}
}

func TestGenerationStoreRequiresIDGenerator(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("missing ID generator must fail at wiring time")
		}
	}()
	NewGenerationBatchStore(nil, nil)
}

func TestValidateProfileEvidenceRequiresSuppliedFileAndChineseReason(t *testing.T) {
	source := "FILE: package.json\n{\"scripts\":{}}\n\n---\n\nFILE: go.mod\nmodule example"
	if err := validateProfileEvidence([]string{"package.json：声明了测试脚本"}, source); err != nil {
		t.Fatalf("expected valid profile evidence: %v", err)
	}
	if err := validateProfileEvidence([]string{"README.md：说明了测试方式"}, source); err == nil {
		t.Fatal("expected unsupplied file to be rejected")
	}
}
