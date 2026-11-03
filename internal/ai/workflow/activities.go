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

func generationContext(_ context.Context, input BatchActivityInput) context.Context {
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
	actCtx := generationContext(ctx, input)
	if err := transitionGeneration(actCtx, input, domain.GenerationFetching); err != nil {
		return err
	}
	return generationDeps.store.FreezeSource(actCtx, input.TeamID, input.BatchID)
}

func ParseAndNormalize(ctx context.Context, input BatchActivityInput) error {
	actCtx := generationContext(ctx, input)
	if err := transitionGeneration(actCtx, input, domain.GenerationParsing); err != nil {
		return err
	}
	return generationDeps.store.NormalizeSourceSnapshot(actCtx, input.TeamID, input.BatchID)
}

func GenerateSeeds(ctx context.Context, input BatchActivityInput) error {
	actCtx := generationContext(ctx, input)
	if err := transitionGeneration(actCtx, input, domain.GenerationGenerating); err != nil {
		return err
	}
	return generationDeps.store.GenerateModelSeed(actCtx, input.TeamID, input.BatchID, generationDeps.gen.Next())
}

func QualityGate(ctx context.Context, input BatchActivityInput) error {
	actCtx := generationContext(ctx, input)
	if err := transitionGeneration(actCtx, input, domain.GenerationQualityCheck); err != nil {
		return err
	}
	if err := generationDeps.store.QualityCheckSeeds(actCtx, input.TeamID, input.BatchID); err != nil {
		return err
	}
	// The durable record is an evidence envelope, not a second source-code
	// repository. This runs only after the model and deterministic gate have
	// consumed the bounded source bodies.
	if err := generationDeps.store.PurgeSourceContents(actCtx, input.TeamID, input.BatchID); err != nil {
		return err
	}
	return transitionGeneration(actCtx, input, domain.GenerationReview)
}

func FailGeneration(ctx context.Context, input BatchActivityInput) error {
	actCtx := generationContext(ctx, input)
	batch, err := generationDeps.svc.Find(actCtx, input.TeamID, input.BatchID)
	if err != nil {
		return err
	}
	if batch.State == domain.GenerationApproved || batch.State == domain.GenerationRejected {
		return nil
	}
	if err := generationDeps.store.RecordFailureStage(actCtx, input.TeamID, input.BatchID, input.FailureStage); err != nil {
		return err
	}
	if batch.State == domain.GenerationFailed {
		return nil
	}
	_, err = generationDeps.svc.Transition(actCtx, input.TeamID, input.BatchID, batch.StateVersion, domain.GenerationFailed)
	return err
}

func RecordReview(_ context.Context, payload ReviewSignalPayload) error {
	if generationDeps == nil {
		return fmt.Errorf("generation dependencies are not configured")
	}
	return generationDeps.store.RecordReview(rls.WithTenant(context.Background(), payload.TeamID), payload.TeamID, payload.BatchID, payload.ActorID, payload.StateVersion, payload.Decision, payload.RequestID)
}

func CommitVersions(_ context.Context, payload ReviewSignalPayload) error {
	if generationDeps == nil {
		return fmt.Errorf("generation dependencies are not configured")
	}
	actCtx := rls.WithTenant(context.Background(), payload.TeamID)
	// Review is a human decision, but the final commit boundary must still
	// re-run the deterministic quality gate. This protects against legacy or
	// manually altered batches that reached review with stale "passed" flags.
	if err := generationDeps.store.QualityCheckSeeds(actCtx, payload.TeamID, payload.BatchID); err != nil {
		return err
	}
	if err := generationDeps.store.RecordReview(actCtx, payload.TeamID, payload.BatchID, payload.ActorID, payload.StateVersion, "approve", payload.RequestID); err != nil {
		return err
	}
	if err := generationDeps.store.CommitSeeds(actCtx, payload.TeamID, payload.BatchID, payload.ActorID, generationDeps.gen.Next); err != nil {
		return err
	}
	batch, err := generationDeps.svc.Find(actCtx, payload.TeamID, payload.BatchID)
	if err != nil {
		return err
	}
	if batch.State == domain.GenerationApproved {
		return nil
	}
	_, err = generationDeps.svc.Transition(actCtx, payload.TeamID, payload.BatchID, batch.StateVersion, domain.GenerationApproved)
	return err
}

func ApplyRollback(_ context.Context, payload RollbackSignalPayload) error {
	if generationDeps == nil {
		return fmt.Errorf("generation dependencies are not configured")
	}
	return generationDeps.store.RecordReview(rls.WithTenant(context.Background(), payload.TeamID), payload.TeamID, payload.BatchID, payload.ActorID, payload.StateVersion, "rollback", payload.RequestID)
}

func EmitAudit(ctx context.Context, payload ReviewSignalPayload) error {
	activity.GetLogger(ctx).Info("generation batch approved", "batch_id", payload.BatchID)
	return nil
}
