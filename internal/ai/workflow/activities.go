package workflow

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

var generationDeps *generationActivities

type generationActivities struct {
	svc   *application.GenerationBatchService
	store *infra.GenerationBatchStore
	gen   *id.Generator
}

func SetGenerationDeps(svc *application.GenerationBatchService, store *infra.GenerationBatchStore, gen *id.Generator) {
	generationDeps = &generationActivities{svc: svc, store: store, gen: gen}
}

func generationContext(ctx context.Context, input BatchActivityInput) context.Context {
	return rls.WithTenant(context.Background(), input.TeamID)
}

func transitionGeneration(ctx context.Context, input BatchActivityInput, next domain.GenerationBatchState) error {
	batch, err := generationDeps.svc.Find(ctx, input.TeamID, input.BatchID)
	if err != nil {
		return err
	}
	// Temporal retries may re-enter an activity after its state transition was
	// committed. Treat the requested state as idempotently complete.
	if batch.State == next {
		return nil
	}
	_, err = generationDeps.svc.Transition(ctx, input.TeamID, input.BatchID, batch.StateVersion, next)
	return err
}

func FetchSource(ctx context.Context, input BatchActivityInput) error {
	return transitionGeneration(generationContext(ctx, input), input, domain.GenerationFetching)
}

func ParseAndNormalize(ctx context.Context, input BatchActivityInput) error {
	return transitionGeneration(generationContext(ctx, input), input, domain.GenerationParsing)
}

func GenerateSeeds(ctx context.Context, input BatchActivityInput) error {
	actCtx := generationContext(ctx, input)
	if err := transitionGeneration(actCtx, input, domain.GenerationGenerating); err != nil {
		return err
	}
	// Development adapter: persist a deterministic candidate seed from the
	// frozen source identifiers. Production adapters replace this payload with
	// normalized SCM/OpenAPI/requirement output without changing the workflow.
	spec := map[string]any{"title": fmt.Sprintf("批次 %d 生成候选用例", input.BatchID), "kind": "api", "batch_id": input.BatchID}
	return generationDeps.store.CreateSeed(actCtx, input.TeamID, generationDeps.gen.Next(), input.BatchID, "dev://generation/"+itoa(input.BatchID), spec)
}

func QualityGate(ctx context.Context, input BatchActivityInput) error {
	actCtx := generationContext(ctx, input)
	if err := transitionGeneration(actCtx, input, domain.GenerationQualityCheck); err != nil {
		return err
	}
	if err := generationDeps.store.MarkSeedsQuality(actCtx, input.TeamID, input.BatchID, "passed"); err != nil {
		return err
	}
	return transitionGeneration(actCtx, input, domain.GenerationReview)
}

func RecordReview(ctx context.Context, payload ReviewSignalPayload) error {
	if generationDeps == nil {
		return fmt.Errorf("generation dependencies are not configured")
	}
	return nil
}

func CommitVersions(ctx context.Context, payload ReviewSignalPayload) error {
	if generationDeps == nil {
		return fmt.Errorf("generation dependencies are not configured")
	}
	actCtx := rls.WithTenant(context.Background(), payload.TeamID)
	if err := generationDeps.store.CommitSeeds(actCtx, payload.TeamID, payload.BatchID, generationDeps.gen.Next); err != nil {
		return err
	}
	batch, err := generationDeps.svc.Find(actCtx, payload.TeamID, payload.BatchID)
	if err != nil {
		return err
	}
	_, err = generationDeps.svc.Transition(actCtx, payload.TeamID, payload.BatchID, batch.StateVersion, domain.GenerationApproved)
	return err
}

func ApplyRollback(ctx context.Context, payload RollbackSignalPayload) error { return nil }
func EmitAudit(ctx context.Context, payload ReviewSignalPayload) error {
	activity.GetLogger(ctx).Info("generation batch approved", "batch_id", payload.BatchID)
	return nil
}
