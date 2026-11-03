package infra

import (
	"context"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
)

// GenerationCostReadAdapter implements the AI domain cost read port over the
// trusted provider-owned BatchCostReader contract. AI may import trusted
// application (consumer → provider); trusted never imports AI.
type GenerationCostReadAdapter struct {
	reader trustedapp.BatchCostReader
}

func NewGenerationCostReadAdapter(reader trustedapp.BatchCostReader) *GenerationCostReadAdapter {
	return &GenerationCostReadAdapter{reader: reader}
}

var _ domain.GenerationCostReadPort = (*GenerationCostReadAdapter)(nil)

// BatchCosts delegates one batched trusted read; the trusted service injects the
// tenant scope and counts exact batch_id matches only.
func (a *GenerationCostReadAdapter) BatchCosts(ctx context.Context, teamID int64, batchIDs []int64) (map[int64]float64, error) {
	return a.reader.BatchCosts(ctx, teamID, batchIDs)
}
