package domain

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrGenerationRepositoryNotFound means the repository is absent from the
	// tenant's project or was deleted.
	ErrGenerationRepositoryNotFound = errors.New("generation repository not found")
	// ErrGenerationRepositoryRefNotFound means the selected ref was never
	// synced or has been removed by a refresh.
	ErrGenerationRepositoryRefNotFound = errors.New("generation repository ref not found")
)

// GenerationPlanRepository persists preflight plans. FindIdempotent returns the
// newest confirmable draft with an equal hash so a refresh/retry of the same
// request does not create plan rows forever. Consume is a single-row CAS that
// must succeed before a run is created: a plan can back at most one run.
type GenerationPlanRepository interface {
	Create(ctx context.Context, plan *GenerationPlan) error
	Find(ctx context.Context, teamID, planID int64) (*GenerationPlan, error)
	FindIdempotent(ctx context.Context, teamID int64, planHash string, now time.Time) (*GenerationPlan, error)
	// Consume atomically moves a draft, unexpired plan to consumed and links it
	// to the run. It returns false when another request already consumed the
	// plan or it expired; callers must then return 409 plan_stale.
	Consume(ctx context.Context, teamID, planID, runID int64, now time.Time) (bool, error)
	ExpireBefore(ctx context.Context, now time.Time, limit int) (int64, error)
}

// GenerationModelBinding is the project's effective generation model. The
// billing key in the cost ledger is derived from it (the DSH runtime bills as
// "dsh/<model_key>"), so provider and model stay separate: preflight tries the
// real runtime key before compatibility fallbacks.
type GenerationModelBinding struct {
	Provider string
	Model    string
}

// GenerationRepositoryReadiness is the read-only catalog view of one repository
// and its SCM connection. Preflight never performs an SCM network call itself.
type GenerationRepositoryReadiness struct {
	RepoURL       string
	SCMConfigured bool
	SCMStatus     string
}

// GenerationRepositoryRef is a server-cached SCM reference selected for
// generation. CommitSHA is informational; the batch pipeline re-freezes source.
type GenerationRepositoryRef struct {
	RefType   string
	Name      string
	CommitSHA string
	FetchedAt time.Time
}

// GenerationUsageSample is the historical approved-batch sample used for
// estimation. SampleCount==0 means no approved history exists.
type GenerationUsageSample struct {
	SampleCount int
	TokensIn    int64
	TokensOut   int64
	// DurationSeconds is the average measured wall time of approved batches.
	DurationSeconds int
	// CandidatesMin/Max are the observed candidate counts, not the quality gate.
	CandidatesMin int
	CandidatesMax int
}

// GenerationModelPrice is the effective token tariff of the model bound to the
// project. Unit prices are per 1K tokens, matching the trusted cost ledger.
type GenerationModelPrice struct {
	Model           string
	InputUnitPrice  float64
	OutputUnitPrice float64
	Currency        string
}

// GenerationPlanCatalogPort is the read-only governance/cost surface preflight
// depends on. Implementations belong to infra adapters; the application never
// queries governance tables or the trusted ledger directly.
type GenerationPlanCatalogPort interface {
	ReadGenerationScope(ctx context.Context, teamID, projectID, serviceID int64) (*GenerationScope, error)
	ReadRepositoryReadiness(ctx context.Context, teamID, projectID, repoID int64) (*GenerationRepositoryReadiness, error)
	ReadRepositoryRef(ctx context.Context, teamID, repoID int64, refType, name string) (*GenerationRepositoryRef, error)
	FindLatestApprovedBaseline(ctx context.Context, teamID, projectID, repoID, serviceID int64) (*int64, error)
	ReadServiceUsage(ctx context.Context, teamID, projectID, repoID, serviceID int64) (*GenerationUsageSample, error)
	ReadFallbackUsage(ctx context.Context, teamID, projectID int64) (*GenerationUsageSample, error)
	// ReadGenerationModel returns the model bound to the project. Preflight
	// derives the real billing keys from it; an empty binding means no model.
	ReadGenerationModel(ctx context.Context, teamID, projectID int64) (GenerationModelBinding, error)
}

// GenerationPlanPricePort resolves the effective tariff for a model. It is
// implemented by the trusted-domain adapter so the AI domain never reads the
// cost ledger itself.
type GenerationPlanPricePort interface {
	EffectiveGenerationPrice(ctx context.Context, model string) (*GenerationModelPrice, error)
}

// The generation run repository contract lives in generation_run.go because the
// run and its units are one aggregate; this file only owns the plan/catalog
// contracts.
