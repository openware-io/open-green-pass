package application

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type fakePlanCatalog struct {
	scopes    map[int64]*domain.GenerationScope
	readiness map[int64]*domain.GenerationRepositoryReadiness
	refs      map[string]*domain.GenerationRepositoryRef
	baselines map[int64]int64
	usage     map[int64]*domain.GenerationUsageSample
	fallback  *domain.GenerationUsageSample
	model     domain.GenerationModelBinding
	err       error
}

func newFakePlanCatalog() *fakePlanCatalog {
	return &fakePlanCatalog{
		scopes: map[int64]*domain.GenerationScope{}, readiness: map[int64]*domain.GenerationRepositoryReadiness{},
		refs: map[string]*domain.GenerationRepositoryRef{}, baselines: map[int64]int64{},
		usage: map[int64]*domain.GenerationUsageSample{},
		model: domain.GenerationModelBinding{Provider: "deepseek", Model: "deepseek-flash"},
	}
}

func fakePlanRefKey(repoID int64, refType, name string) string {
	return strings.Join([]string{refType, name, strconv.FormatInt(repoID, 10)}, "|")
}

func (c *fakePlanCatalog) ReadGenerationScope(_ context.Context, _, _, serviceID int64) (*domain.GenerationScope, error) {
	if c.err != nil {
		return nil, c.err
	}
	scope, ok := c.scopes[serviceID]
	if !ok {
		return nil, domain.ErrGenerationNotFound
	}
	copied := *scope
	return &copied, nil
}

func (c *fakePlanCatalog) ReadRepositoryReadiness(_ context.Context, _, _, repoID int64) (*domain.GenerationRepositoryReadiness, error) {
	if c.err != nil {
		return nil, c.err
	}
	readiness, ok := c.readiness[repoID]
	if !ok {
		return nil, domain.ErrGenerationRepositoryNotFound
	}
	copied := *readiness
	return &copied, nil
}

func (c *fakePlanCatalog) ReadRepositoryRef(_ context.Context, _, repoID int64, refType, name string) (*domain.GenerationRepositoryRef, error) {
	if c.err != nil {
		return nil, c.err
	}
	ref, ok := c.refs[fakePlanRefKey(repoID, refType, name)]
	if !ok {
		return nil, domain.ErrGenerationRepositoryRefNotFound
	}
	copied := *ref
	return &copied, nil
}

func (c *fakePlanCatalog) FindLatestApprovedBaseline(_ context.Context, _, _, _, serviceID int64) (*int64, error) {
	if c.err != nil {
		return nil, c.err
	}
	if id, ok := c.baselines[serviceID]; ok {
		return &id, nil
	}
	return nil, nil
}

func (c *fakePlanCatalog) ReadServiceUsage(_ context.Context, _, _, _, serviceID int64) (*domain.GenerationUsageSample, error) {
	if c.err != nil {
		return nil, c.err
	}
	if sample, ok := c.usage[serviceID]; ok {
		copied := *sample
		return &copied, nil
	}
	return &domain.GenerationUsageSample{}, nil
}

func (c *fakePlanCatalog) ReadFallbackUsage(_ context.Context, _, _ int64) (*domain.GenerationUsageSample, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.fallback == nil {
		return &domain.GenerationUsageSample{}, nil
	}
	copied := *c.fallback
	return &copied, nil
}

func (c *fakePlanCatalog) ReadGenerationModel(_ context.Context, _, _ int64) (domain.GenerationModelBinding, error) {
	return c.model, c.err
}

type fakeRunScopeResolver struct {
	items []domain.GenerationRunScope
	err   error
}

func (r fakeRunScopeResolver) ResolveGenerationRunScope(context.Context, int64, int64, string, int64) ([]domain.GenerationRunScope, error) {
	return r.items, r.err
}

// fakePlanPrice records every requested billing key and can return a price only
// for an expected key, so tests fail if preflight uses the wrong model key.
type fakePlanPrice struct {
	price         *domain.GenerationModelPrice
	err           error
	expectedModel string
	models        []string
}

func (p *fakePlanPrice) EffectiveGenerationPrice(_ context.Context, model string) (*domain.GenerationModelPrice, error) {
	p.models = append(p.models, model)
	if p.err != nil {
		return nil, p.err
	}
	if p.expectedModel != "" && model != p.expectedModel {
		return nil, nil
	}
	return p.price, nil
}

func addPlanScope(catalog *fakePlanCatalog, serviceID, repoID int64) {
	catalog.scopes[serviceID] = &domain.GenerationScope{
		ServiceID: serviceID, TeamID: 10, TargetID: 20, RepoID: repoID, SourcePath: "services/svc",
		Status: "active", GenerationStatus: "confirmed", ProfileStatus: "confirmed",
		Language: "Go", Framework: "net/http",
	}
	catalog.readiness[repoID] = &domain.GenerationRepositoryReadiness{RepoURL: "https://github.com/acme/repo", SCMConfigured: true, SCMStatus: "configured"}
	catalog.refs[fakePlanRefKey(repoID, "branch", "main")] = &domain.GenerationRepositoryRef{RefType: "branch", Name: "main", CommitSHA: "deadbeef", FetchedAt: time.Now().UTC()}
}

func newTestPlanService(t *testing.T, catalog *fakePlanCatalog) (*GenerationPlanService, *infra.MemoryGenerationPlanRepository) {
	t.Helper()
	gen, err := id.New(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	planRepo := infra.NewMemoryGenerationPlanRepository()
	svc := NewGenerationPlanService(planRepo, catalog, fakeRunScopeResolver{}, gen)
	return svc, planRepo
}

func TestGenerationPlanPreflightIsValidAndHashOrderIndependent(t *testing.T) {
	catalog := newFakePlanCatalog()
	addPlanScope(catalog, 50, 30)
	addPlanScope(catalog, 51, 31)
	svc, _ := newTestPlanService(t, catalog)
	units := []domain.GenerationPlanUnitInput{
		{ServiceID: 50, RepoID: 30, SourcePath: "services/svc", RefType: "branch", RefName: "main"},
		{ServiceID: 51, RepoID: 31, SourcePath: "services/svc", RefType: "branch", RefName: "main"},
	}
	first, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{TeamID: 10, ProjectID: 20, ScopeType: "project", ActorID: 40, Units: units})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.PlanHash, "sha256:") || first.Summary.Services != 2 || first.Summary.Batches != 2 {
		t.Fatalf("unexpected plan: %+v", first)
	}
	if !first.Estimate.Unknown || first.Estimate.TokensIn != 0 {
		t.Fatalf("estimate must be unknown without history: %+v", first.Estimate)
	}
	for _, item := range first.Items {
		if !item.Valid || item.CommitSHA != "deadbeef" {
			t.Fatalf("unexpected item: %+v", item)
		}
	}
	// Reordering units must not change the fingerprint and must reuse the plan.
	reversed := []domain.GenerationPlanUnitInput{units[1], units[0]}
	second, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{TeamID: 10, ProjectID: 20, ScopeType: "project", ActorID: 40, Units: reversed})
	if err != nil {
		t.Fatal(err)
	}
	if second.PlanHash != first.PlanHash {
		t.Fatalf("plan hash depends on unit order: %s != %s", first.PlanHash, second.PlanHash)
	}
	if second.ID != first.ID {
		t.Fatalf("identical preflight did not reuse the plan: %d != %d", second.ID, first.ID)
	}
}

func TestGenerationPlanPreflightReportsInvalidReasons(t *testing.T) {
	catalog := newFakePlanCatalog()
	addPlanScope(catalog, 50, 30)
	addPlanScope(catalog, 51, 31)
	catalog.scopes[52] = &domain.GenerationScope{
		ServiceID: 52, TeamID: 10, TargetID: 20, RepoID: 32, SourcePath: "services/pending",
		Status: "active", GenerationStatus: "pending", ProfileStatus: "confirmed", Language: "Go", Framework: "x",
	}
	catalog.readiness[32] = &domain.GenerationRepositoryReadiness{RepoURL: "https://github.com/acme/pending", SCMConfigured: true}
	catalog.refs[fakePlanRefKey(32, "branch", "main")] = &domain.GenerationRepositoryRef{RefType: "branch", Name: "main", CommitSHA: "c", FetchedAt: time.Now().UTC()}
	svc, _ := newTestPlanService(t, catalog)
	plan, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{
		TeamID: 10, ProjectID: 20, ScopeType: "project", ActorID: 40,
		Units: []domain.GenerationPlanUnitInput{
			{ServiceID: 50, RepoID: 30, SourcePath: "services/svc", RefType: "branch", RefName: "main"},
			{ServiceID: 51, RepoID: 31, SourcePath: "services/svc", RefType: "branch", RefName: "missing"},
			{ServiceID: 52, RepoID: 32, SourcePath: "services/pending", RefType: "branch", RefName: "main"},
			{ServiceID: 53, RepoID: 33, SourcePath: ".", RefType: "branch", RefName: "main"},
			{ServiceID: 50, RepoID: 30, SourcePath: "services/svc", RefType: "branch", RefName: "main"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Services != 1 || plan.Summary.Batches != 1 {
		t.Fatalf("summary = %+v", plan.Summary)
	}
	codes := map[string]int{}
	for _, item := range plan.Items {
		if item.Valid {
			continue
		}
		code := strings.SplitN(item.Error, ":", 2)[0]
		codes[code]++
	}
	for _, want := range []string{"ref_not_found", "service_not_confirmed", "service_not_found", "duplicate_service"} {
		if codes[want] == 0 {
			t.Fatalf("invalid code %q missing: %v", want, codes)
		}
	}
}

//nolint:gocyclo // Scenario fixture intentionally covers estimate fallback/price matrix.
func TestGenerationPlanEstimateUsesApprovedHistoryAndPrice(t *testing.T) {
	catalog := newFakePlanCatalog()
	addPlanScope(catalog, 50, 30)
	catalog.usage[50] = &domain.GenerationUsageSample{SampleCount: 3, TokensIn: 3000, TokensOut: 3500, DurationSeconds: 90, CandidatesMin: 6, CandidatesMax: 6}
	catalog.baselines[50] = 777
	svc, _ := newTestPlanService(t, catalog)
	price := &fakePlanPrice{
		price:         &domain.GenerationModelPrice{Model: "dsh/deepseek-flash", InputUnitPrice: 0.01, OutputUnitPrice: 0.02, Currency: "CNY"},
		expectedModel: "dsh/deepseek-flash",
	}
	svc.SetPricePort(price)
	plan, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{
		TeamID: 10, ProjectID: 20, ScopeType: "project", ActorID: 40,
		Units: []domain.GenerationPlanUnitInput{{ServiceID: 50, RepoID: 30, SourcePath: "services/svc", RefType: "branch", RefName: "main"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(price.models) == 0 || price.models[0] != "dsh/deepseek-flash" {
		t.Fatalf("preflight did not use the DSH billing key first: %v", price.models)
	}
	if len(plan.Risks) != 0 {
		for _, risk := range plan.Risks {
			if risk.Code == "price_missing" {
				t.Fatalf("price_missing despite configured dsh price: %v", plan.Risks)
			}
		}
	}
	estimate := plan.Estimate
	if estimate.Unknown || estimate.TokensIn != 3000 || estimate.TokensOut != 3500 || estimate.DurationSeconds != 90 {
		t.Fatalf("estimate = %+v", estimate)
	}
	if estimate.CostAmount < 0.099 || estimate.CostAmount > 0.101 || estimate.Currency != "CNY" {
		t.Fatalf("cost = %v %s", estimate.CostAmount, estimate.Currency)
	}
	if plan.Summary.CandidatesMin != 6 || plan.Summary.CandidatesMax != 6 {
		t.Fatalf("candidates = %d..%d", plan.Summary.CandidatesMin, plan.Summary.CandidatesMax)
	}
}

func TestGenerationPlanPreflightValidatesScopeAndSize(t *testing.T) {
	catalog := newFakePlanCatalog()
	svc, _ := newTestPlanService(t, catalog)
	if _, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{TeamID: 10, ProjectID: 20, ScopeType: "project", ActorID: 40}); !errors.Is(err, ErrGenerationPlanValidation) {
		t.Fatalf("empty scope error = %v", err)
	}
	units := make([]domain.GenerationPlanUnitInput, 0, 101)
	for i := int64(1); i <= 101; i++ {
		units = append(units, domain.GenerationPlanUnitInput{ServiceID: i, RepoID: 30, RefType: "branch", RefName: "main"})
	}
	if _, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{TeamID: 10, ProjectID: 20, ScopeType: "project", ActorID: 40, Units: units}); !errors.Is(err, ErrGenerationPlanValidation) {
		t.Fatalf("too many services error = %v", err)
	}
}

func TestGenerationPlanPreflightResolvesProjectScope(t *testing.T) {
	catalog := newFakePlanCatalog()
	addPlanScope(catalog, 50, 30)
	addPlanScope(catalog, 51, 31)
	gen, err := id.New(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	planRepo := infra.NewMemoryGenerationPlanRepository()
	resolver := fakeRunScopeResolver{items: []domain.GenerationRunScope{
		{ServiceID: 50, RepoID: 30, SourcePath: "services/svc", DefaultBranch: "main"},
		{ServiceID: 51, RepoID: 31, SourcePath: "services/svc", DefaultBranch: "main"},
	}}
	svc := NewGenerationPlanService(planRepo, catalog, resolver, gen)
	plan, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{TeamID: 10, ProjectID: 20, ScopeType: "project", ActorID: 40})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Services != 2 || len(plan.Items) != 2 {
		t.Fatalf("resolved plan = %+v", plan)
	}
}

func TestGenerationPlanPreflightRejectsUnitsOutsideScope(t *testing.T) {
	catalog := newFakePlanCatalog()
	addPlanScope(catalog, 50, 30)
	addPlanScope(catalog, 51, 31)
	svc, _ := newTestPlanService(t, catalog)
	cases := []struct {
		name  string
		scope string
		id    int64
		unit  domain.GenerationPlanUnitInput
	}{
		{name: "repo scope with other repo", scope: "repo", id: 30, unit: domain.GenerationPlanUnitInput{ServiceID: 51, RepoID: 31, SourcePath: "services/svc", RefType: "branch", RefName: "main"}},
		{name: "service scope with other service", scope: "service", id: 50, unit: domain.GenerationPlanUnitInput{ServiceID: 51, RepoID: 31, SourcePath: "services/svc", RefType: "branch", RefName: "main"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{
				TeamID: 10, ProjectID: 20, ScopeType: tc.scope, ScopeID: tc.id, ActorID: 40,
				Units: []domain.GenerationPlanUnitInput{tc.unit},
			})
			if err != nil {
				t.Fatal(err)
			}
			if plan.Summary.Services != 0 || len(plan.Items) != 1 || plan.Items[0].Valid {
				t.Fatalf("scope mismatch accepted: %+v", plan)
			}
			if code := strings.SplitN(plan.Items[0].Error, ":", 2)[0]; code != "scope_mismatch" {
				t.Fatalf("invalid code = %q, error=%q", code, plan.Items[0].Error)
			}
		})
	}
	if _, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{
		TeamID: 10, ProjectID: 20, ScopeType: "repo", ScopeID: 0, ActorID: 40,
	}); !errors.Is(err, ErrGenerationPlanValidation) {
		t.Fatalf("repo scope without scope_id error = %v", err)
	}
	if _, err := svc.Preflight(context.Background(), PreflightGenerationPlanCommand{
		TeamID: 10, ProjectID: 20, ScopeType: "project", ScopeID: 99, ActorID: 40,
	}); !errors.Is(err, ErrGenerationPlanValidation) {
		t.Fatalf("project scope with foreign id error = %v", err)
	}
}
