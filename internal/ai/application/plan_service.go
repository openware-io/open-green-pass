package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// PreflightGenerationPlanCommand is one operator preflight request. Units may
// be empty when the operator selected a project/repo scope; the server then
// expands the scope with the same resolver used by run creation.
type PreflightGenerationPlanCommand struct {
	ID        int64
	TeamID    int64
	ProjectID int64
	ScopeType string
	ScopeID   int64
	ActorID   int64
	RequestID string
	Units     []domain.GenerationPlanUnitInput
}

// ErrGenerationPlanValidation marks a request-level preflight failure (as
// opposed to an infrastructure failure), so the HTTP adapter can return 400.
var ErrGenerationPlanValidation = errors.New("generation plan validation failed")

// GenerationPlanService runs read-only preflight checks and persists the
// resulting plan. It never creates runs, batches, or SCM traffic.
type GenerationPlanService struct {
	plans   domain.GenerationPlanRepository
	catalog domain.GenerationPlanCatalogPort
	scope   domain.GenerationRunScopePort
	price   domain.GenerationPlanPricePort
	gen     *id.Generator
	now     func() time.Time
	log     *slog.Logger
}

func NewGenerationPlanService(plans domain.GenerationPlanRepository, catalog domain.GenerationPlanCatalogPort, scope domain.GenerationRunScopePort, gen *id.Generator) *GenerationPlanService {
	return &GenerationPlanService{plans: plans, catalog: catalog, scope: scope, gen: gen, now: func() time.Time { return time.Now().UTC() }, log: slog.Default()}
}

// SetPricePort wires the trusted-domain tariff adapter. Without it estimates
// report price_missing instead of a fabricated cost.
func (s *GenerationPlanService) SetPricePort(price domain.GenerationPlanPricePort) { s.price = price }

// Find returns a persisted plan for replay/audit.
func (s *GenerationPlanService) Find(ctx context.Context, teamID, planID int64) (*domain.GenerationPlan, error) {
	if teamID <= 0 || planID <= 0 {
		return nil, errors.New("generation plan requires tenant and id")
	}
	return s.plans.Find(ctx, teamID, planID)
}

// Preflight validates every unit against the catalog and current cached refs,
// computes a historical-only estimate, and persists a single-use plan.
func (s *GenerationPlanService) Preflight(ctx context.Context, cmd PreflightGenerationPlanCommand) (*domain.GenerationPlan, error) {
	units, err := s.preparePreflight(ctx, &cmd)
	if err != nil {
		return nil, err
	}
	agg := newPreflightAggregate()
	agg.addRisk("review_required", "生成后需人工审核，审核通过才会写入用例库", "info")
	s.collectPreflightItems(ctx, cmd, units, agg)
	estimate := agg.estimate()
	if err := s.applyPreflightPrice(ctx, cmd, &estimate, agg.addRisk); err != nil {
		return nil, err
	}
	plan := buildPreflightPlan(cmd, units, agg, estimate, s.now())
	return s.persistPreflightPlan(ctx, cmd, plan)
}

// preparePreflight validates the dependency/command contract and resolves the
// concrete unit list (explicit units or the selected scope).
func (s *GenerationPlanService) preparePreflight(ctx context.Context, cmd *PreflightGenerationPlanCommand) ([]domain.GenerationPlanUnitInput, error) {
	if err := s.validatePreflightCommand(cmd); err != nil {
		return nil, err
	}
	// Lazy TTL cleanup keeps a tenant's draft plans bounded even when the
	// cross-tenant worker GC cannot enumerate tenant registries.
	if cleaner, ok := s.plans.(interface {
		ExpireBefore(context.Context, time.Time, int) (int64, error)
	}); ok {
		_, _ = cleaner.ExpireBefore(ctx, s.now(), 200)
	}
	units := make([]domain.GenerationPlanUnitInput, 0, len(cmd.Units))
	for _, unit := range cmd.Units {
		units = append(units, domain.EffectiveGenerationPlanUnit(unit))
	}
	if len(units) == 0 {
		resolved, err := s.scope.ResolveGenerationRunScope(ctx, cmd.TeamID, cmd.ProjectID, cmd.ScopeType, cmd.ScopeID)
		if err != nil {
			return nil, err
		}
		if len(resolved) == 0 {
			return nil, fmt.Errorf("%w: empty_scope: no confirmed services match the generation scope", ErrGenerationPlanValidation)
		}
		for _, item := range resolved {
			units = append(units, domain.GenerationPlanUnitInput{ServiceID: item.ServiceID, RepoID: item.RepoID, SourcePath: item.SourcePath, RefType: "branch", RefName: item.DefaultBranch})
		}
	}
	if len(units) > 100 {
		return nil, fmt.Errorf("%w: too_many_services: a generation plan supports at most 100 services", ErrGenerationPlanValidation)
	}
	return units, nil
}

// validatePreflightCommand checks dependencies, command ids and scope shape.
func (s *GenerationPlanService) validatePreflightCommand(cmd *PreflightGenerationPlanCommand) error {
	if s == nil || s.plans == nil {
		return errors.New("generation plan repository is required")
	}
	if s.catalog == nil || s.scope == nil || s.gen == nil {
		return errors.New("generation plan dependencies are not configured")
	}
	if cmd.TeamID <= 0 || cmd.ProjectID <= 0 || cmd.ActorID <= 0 {
		return fmt.Errorf("%w: tenant, project and actor are required", ErrGenerationPlanValidation)
	}
	if cmd.ID <= 0 {
		cmd.ID = s.gen.Next()
	}
	return validatePreflightScope(cmd)
}

func validatePreflightScope(cmd *PreflightGenerationPlanCommand) error {
	if !domain.ValidGenerationScopeType(cmd.ScopeType) {
		return fmt.Errorf("%w: scope must be project, repo or service", ErrGenerationPlanValidation)
	}
	if cmd.ScopeType != "project" && cmd.ScopeID <= 0 {
		return fmt.Errorf("%w: repo and service scopes require scope_id", ErrGenerationPlanValidation)
	}
	if cmd.ScopeType == "project" && cmd.ScopeID != 0 && cmd.ScopeID != cmd.ProjectID {
		return fmt.Errorf("%w: project scope_id must be 0 or the project id", ErrGenerationPlanValidation)
	}
	return nil
}

type preflightAggregate struct {
	items         []domain.GenerationPlanItem
	risks         []domain.GenerationPlanRisk
	riskKeys      map[string]struct{}
	validServices int
	repos         map[int64]struct{}
	knownUnits    int
	tokensIn      int64
	tokensOut     int64
	duration      int
	candidatesMin int
	candidatesMax int
}

func newPreflightAggregate() *preflightAggregate {
	return &preflightAggregate{riskKeys: map[string]struct{}{}, repos: map[int64]struct{}{}}
}

func (a *preflightAggregate) addRisk(code, message, severity string) {
	if _, exists := a.riskKeys[code]; exists {
		return
	}
	a.riskKeys[code] = struct{}{}
	a.risks = append(a.risks, domain.GenerationPlanRisk{Code: code, Message: message, Severity: severity})
}

func (a *preflightAggregate) estimate() domain.GenerationPlanEstimate {
	var estimate domain.GenerationPlanEstimate
	if a.knownUnits == 0 {
		estimate.Unknown = true
		return estimate
	}
	estimate.TokensIn = a.tokensIn
	estimate.TokensOut = a.tokensOut
	estimate.DurationSeconds = a.duration
	return estimate
}

// collectPreflightItems evaluates every unit and accumulates items, risks and
// historical usage for the estimate.
func (s *GenerationPlanService) collectPreflightItems(ctx context.Context, cmd PreflightGenerationPlanCommand, units []domain.GenerationPlanUnitInput, agg *preflightAggregate) {
	seen := map[int64]struct{}{}
	for _, unit := range units {
		if err := validateUnitScopeOwnership(cmd.ScopeType, cmd.ScopeID, unit); err != nil {
			agg.items = append(agg.items, domain.GenerationPlanItem{
				ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
				RefType: unit.RefType, RefName: unit.RefName, Valid: false,
				Error: "scope_mismatch: " + err.Error(),
			})
			continue
		}
		if _, exists := seen[unit.ServiceID]; exists {
			agg.items = append(agg.items, domain.GenerationPlanItem{
				ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
				RefType: unit.RefType, RefName: unit.RefName, Valid: false,
				Error: "duplicate_service: 同一服务在计划中重复出现",
			})
			continue
		}
		seen[unit.ServiceID] = struct{}{}
		item, itemRisks, usage := s.evaluateUnit(ctx, cmd, unit)
		agg.items = append(agg.items, item)
		for _, risk := range itemRisks {
			agg.addRisk(risk.Code, risk.Message, risk.Severity)
		}
		if item.Valid {
			s.addPlanUnitUsage(agg, item, usage)
		}
	}
	if agg.knownUnits > 0 && agg.knownUnits < agg.validServices {
		agg.addRisk("estimate_partial", "部分服务没有历史 approved 批次，估算只覆盖有历史数据的服务", "warning")
	}
}

func (s *GenerationPlanService) addPlanUnitUsage(agg *preflightAggregate, item domain.GenerationPlanItem, usage *domain.GenerationUsageSample) {
	agg.validServices++
	agg.repos[item.RepoID] = struct{}{}
	if usage == nil || usage.SampleCount == 0 {
		agg.candidatesMin += 3
		agg.candidatesMax += 8
		return
	}
	agg.knownUnits++
	agg.tokensIn += usage.TokensIn
	agg.tokensOut += usage.TokensOut
	agg.duration += usage.DurationSeconds
	agg.candidatesMin += usage.CandidatesMin
	agg.candidatesMax += usage.CandidatesMax
}

// applyPreflightPrice resolves the project model/price and fills estimate cost.
func (s *GenerationPlanService) applyPreflightPrice(ctx context.Context, cmd PreflightGenerationPlanCommand, estimate *domain.GenerationPlanEstimate, addRisk func(string, string, string)) error {
	currency := "CNY"
	if s.price == nil {
		addRisk("price_missing", "成本估算未配置，预估成本未知", "warning")
		estimate.Currency = currency
		return nil
	}
	binding, err := s.catalog.ReadGenerationModel(ctx, cmd.TeamID, cmd.ProjectID)
	if err != nil {
		return fmt.Errorf("read generation model: %w", err)
	}
	price, err := resolveGenerationPrice(ctx, s.price, binding)
	if err != nil {
		return fmt.Errorf("read generation price: %w", err)
	}
	if price == nil {
		addRisk("price_missing", "项目未配置可用模型单价，预估成本未知", "warning")
	} else if estimate.TokensIn > 0 || estimate.TokensOut > 0 {
		estimate.CostAmount = float64(estimate.TokensIn)/1000*price.InputUnitPrice + float64(estimate.TokensOut)/1000*price.OutputUnitPrice
		if strings.TrimSpace(price.Currency) != "" {
			currency = price.Currency
		}
	}
	estimate.Currency = currency
	return nil
}

func buildPreflightPlan(cmd PreflightGenerationPlanCommand, units []domain.GenerationPlanUnitInput, agg *preflightAggregate, estimate domain.GenerationPlanEstimate, now time.Time) *domain.GenerationPlan {
	plan := &domain.GenerationPlan{
		ID:        cmd.ID,
		TeamID:    cmd.TeamID,
		ProjectID: cmd.ProjectID,
		ScopeType: cmd.ScopeType,
		ScopeID:   cmd.ScopeID,
		State:     domain.GenerationPlanDraft,
		Request: domain.GenerationPlanRequestSnapshot{
			ProjectID: cmd.ProjectID, ScopeType: cmd.ScopeType, ScopeID: cmd.ScopeID, Units: units,
		},
		Items: agg.items,
		Summary: domain.GenerationPlanSummary{
			Services: agg.validServices, Repos: len(agg.repos), Batches: agg.validServices,
			CandidatesMin: agg.candidatesMin, CandidatesMax: agg.candidatesMax,
		},
		Estimate:  estimate,
		Risks:     agg.risks,
		ExpiresAt: now.Add(domain.DefaultGenerationPlanTTL),
		CreatedBy: cmd.ActorID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan.PlanHash = domain.CanonicalGenerationPlanHash(plan.ProjectID, plan.ScopeType, plan.ScopeID, plan.Items)
	return plan
}

func (s *GenerationPlanService) persistPreflightPlan(ctx context.Context, cmd PreflightGenerationPlanCommand, plan *domain.GenerationPlan) (*domain.GenerationPlan, error) {
	// Reusing a draft plan with the same fingerprint makes a refresh/retry
	// idempotent and keeps the confirmed hash stable.
	if existing, err := s.plans.FindIdempotent(ctx, cmd.TeamID, plan.PlanHash, s.now()); err == nil {
		if s.log != nil {
			s.log.Info("generation plan reused", "team_id", cmd.TeamID, "plan_id", existing.ID, "request_id", cmd.RequestID)
		}
		return existing, nil
	} else if !errors.Is(err, domain.ErrGenerationPlanNotFound) {
		return nil, fmt.Errorf("find idempotent generation plan: %w", err)
	}
	if err := s.plans.Create(ctx, plan); err != nil {
		return nil, fmt.Errorf("create generation plan: %w", err)
	}
	if s.log != nil {
		s.log.Info("generation plan created", "team_id", cmd.TeamID, "plan_id", plan.ID, "valid_services", plan.Summary.Services, "request_id", cmd.RequestID)
	}
	return plan, nil
}

// evaluateUnit validates one service/reference pair. A non-nil usage means the
// estimate has historical data for this unit.
func (s *GenerationPlanService) evaluateUnit(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput) (domain.GenerationPlanItem, []domain.GenerationPlanRisk, *domain.GenerationUsageSample) {
	item := newPlanUnitItem(unit)
	validated, ok := s.validatePlanUnit(ctx, cmd, unit, item)
	if !ok {
		return validated, nil, nil
	}
	return s.evaluatePlanUnitReadiness(ctx, cmd, unit, validated)
}

func newPlanUnitItem(unit domain.GenerationPlanUnitInput) domain.GenerationPlanItem {
	return domain.GenerationPlanItem{
		ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
		RefType: unit.RefType, RefName: unit.RefName,
	}
}

func invalidPlanUnit(item domain.GenerationPlanItem, code, message string) domain.GenerationPlanItem {
	item.Valid = false
	item.Error = code + ": " + message
	return item
}

// validatePlanUnit applies the syntactic and catalog scope checks before any
// repository/ref/usage read.
func (s *GenerationPlanService) validatePlanUnit(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput, item domain.GenerationPlanItem) (domain.GenerationPlanItem, bool) {
	fieldItem, ok := validatePlanUnitFields(unit, item)
	if !ok {
		return fieldItem, false
	}
	return s.validatePlanUnitScope(ctx, cmd, unit, fieldItem)
}

func validatePlanUnitFields(unit domain.GenerationPlanUnitInput, item domain.GenerationPlanItem) (domain.GenerationPlanItem, bool) {
	if unit.ServiceID <= 0 || unit.RepoID <= 0 || strings.TrimSpace(unit.RefName) == "" {
		return invalidPlanUnit(item, "validation_failed", "服务、仓库和代码引用不能为空"), false
	}
	if unit.RefType != "branch" && unit.RefType != "tag" {
		return invalidPlanUnit(item, "validation_failed", "代码引用类型必须是 branch 或 tag"), false
	}
	return item, true
}

func (s *GenerationPlanService) validatePlanUnitScope(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput, item domain.GenerationPlanItem) (domain.GenerationPlanItem, bool) {
	scope, ok := s.readPlanUnitCatalogScope(ctx, cmd, unit, item)
	if !ok {
		return scope, false
	}
	return validatePlanUnitSourcePath(scope, unit)
}

func (s *GenerationPlanService) readPlanUnitCatalogScope(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput, item domain.GenerationPlanItem) (domain.GenerationPlanItem, bool) {
	scope, err := s.catalog.ReadGenerationScope(ctx, cmd.TeamID, cmd.ProjectID, unit.ServiceID)
	if err != nil || scope == nil {
		return invalidPlanUnit(item, "service_not_found", "当前项目中不存在该目录服务"), false
	}
	if scope.TeamID != cmd.TeamID || scope.TargetID != cmd.ProjectID || scope.ServiceID != unit.ServiceID {
		return invalidPlanUnit(item, "service_not_found", "目录服务不属于所选项目"), false
	}
	if scope.RepoID != unit.RepoID {
		return invalidPlanUnit(item, "service_repo_mismatch", "目录服务不属于所选仓库"), false
	}
	if scope.Status != "active" || scope.GenerationStatus != "confirmed" || scope.ProfileStatus != "confirmed" {
		return invalidPlanUnit(item, "service_not_confirmed", "请先启用目录服务并确认技术画像与用例生成范围"), false
	}
	if strings.TrimSpace(scope.Language) == "" || strings.TrimSpace(scope.Framework) == "" {
		return invalidPlanUnit(item, "profile_incomplete", "目录服务的技术画像不完整，请补充语言和框架"), false
	}
	item.SourcePath = scope.SourcePath
	return item, true
}

func validatePlanUnitSourcePath(item domain.GenerationPlanItem, unit domain.GenerationPlanUnitInput) (domain.GenerationPlanItem, bool) {
	canonical, pathErr := generationSourcePath(item.SourcePath)
	if pathErr != nil {
		return invalidPlanUnit(item, "invalid_source_path", "目录服务的源码目录无效，请先修正目录登记"), false
	}
	requestedPath := unit.SourcePath
	if strings.TrimSpace(requestedPath) == "" {
		requestedPath = item.SourcePath
	}
	if requested, requestedErr := generationSourcePath(requestedPath); requestedErr != nil || requested != canonical {
		return invalidPlanUnit(item, "invalid_source_path", "请求的源码目录与已确认的目录服务不一致"), false
	}
	return item, true
}

// evaluatePlanUnitReadiness reads repo/ref/baseline/usage and returns the item
// plus estimate risks.
func (s *GenerationPlanService) evaluatePlanUnitReadiness(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput, item domain.GenerationPlanItem) (domain.GenerationPlanItem, []domain.GenerationPlanRisk, *domain.GenerationUsageSample) {
	repoItem, ok := s.readPlanUnitRepo(ctx, cmd, unit, item)
	if !ok {
		return repoItem, nil, nil
	}
	item = repoItem
	refItem, stale, ok := s.readPlanUnitRef(ctx, cmd, unit, item)
	if !ok {
		return refItem, nil, nil
	}
	item = refItem
	var risks []domain.GenerationPlanRisk
	if stale {
		risks = append(risks, domain.GenerationPlanRisk{Code: "refs_stale", Message: "代码引用缓存超过 24 小时未刷新，建议先刷新引用", Severity: "warning"})
	}
	if _, err := s.catalog.FindLatestApprovedBaseline(ctx, cmd.TeamID, cmd.ProjectID, unit.RepoID, unit.ServiceID); err != nil {
		return invalidPlanUnit(item, "baseline_unavailable", "系统基线读取失败"), nil, nil
	}
	usage, usageRisks, ok := s.readPlanUnitUsage(ctx, cmd, unit)
	if !ok {
		return invalidPlanUnit(item, "estimate_unavailable", "历史用量读取失败"), nil, nil
	}
	risks = append(risks, usageRisks...)
	item.Valid = true
	return item, risks, usage
}

func (s *GenerationPlanService) readPlanUnitRepo(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput, item domain.GenerationPlanItem) (domain.GenerationPlanItem, bool) {
	readiness, err := s.catalog.ReadRepositoryReadiness(ctx, cmd.TeamID, cmd.ProjectID, unit.RepoID)
	if errors.Is(err, domain.ErrGenerationRepositoryNotFound) || (err == nil && readiness == nil) {
		return invalidPlanUnit(item, "repo_not_found", "仓库不存在或已删除"), false
	}
	if err != nil {
		return invalidPlanUnit(item, "repo_unreadable", "仓库读取失败"), false
	}
	item.RepoURL = readiness.RepoURL
	if !readiness.SCMConfigured {
		return invalidPlanUnit(item, "scm_not_verified", "仓库未绑定可用 SCM 连接，请先配置读取连接"), false
	}
	return item, true
}

func (s *GenerationPlanService) readPlanUnitRef(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput, item domain.GenerationPlanItem) (domain.GenerationPlanItem, bool, bool) {
	ref, err := s.catalog.ReadRepositoryRef(ctx, cmd.TeamID, unit.RepoID, unit.RefType, unit.RefName)
	if errors.Is(err, domain.ErrGenerationRepositoryRefNotFound) || (err == nil && ref == nil) {
		return invalidPlanUnit(item, "ref_not_found", "代码引用未同步或已失效，请先刷新分支和 Tag"), false, false
	}
	if err != nil {
		return invalidPlanUnit(item, "refs_unavailable", "代码引用读取失败"), false, false
	}
	item.CommitSHA = ref.CommitSHA
	stale := !ref.FetchedAt.IsZero() && s.now().Sub(ref.FetchedAt) > 24*time.Hour
	return item, stale, true
}

func (s *GenerationPlanService) readPlanUnitUsage(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput) (*domain.GenerationUsageSample, []domain.GenerationPlanRisk, bool) {
	usage, err := s.catalog.ReadServiceUsage(ctx, cmd.TeamID, cmd.ProjectID, unit.RepoID, unit.ServiceID)
	if err != nil {
		return nil, nil, false
	}
	if usage == nil || usage.SampleCount == 0 {
		fallback, fallbackErr := s.catalog.ReadFallbackUsage(ctx, cmd.TeamID, cmd.ProjectID)
		if fallbackErr != nil {
			return nil, nil, false
		}
		if fallback == nil || fallback.SampleCount == 0 {
			return nil, nil, true
		}
		usage = fallback
		normalizeUsageCandidates(usage)
		return usage, []domain.GenerationPlanRisk{{Code: "estimate_fallback", Message: "该服务无历史 approved 批次，估算使用项目历史均值", Severity: "warning"}}, true
	}
	normalizeUsageCandidates(usage)
	return usage, nil, true
}

func normalizeUsageCandidates(usage *domain.GenerationUsageSample) {
	if usage.CandidatesMin <= 0 {
		usage.CandidatesMin = 3
	}
	if usage.CandidatesMax < usage.CandidatesMin {
		usage.CandidatesMax = usage.CandidatesMin
	}
}

// validateUnitScopeOwnership enforces that an explicit unit belongs to the
// selected scope. Project scope membership is proven by the catalog read in
// evaluateUnit; repo/service scope must additionally match the selected id.
func validateUnitScopeOwnership(scopeType string, scopeID int64, unit domain.GenerationPlanUnitInput) error {
	switch scopeType {
	case "repo":
		if unit.RepoID != scopeID {
			return fmt.Errorf("repo %d is outside the selected repo scope %d", unit.RepoID, scopeID)
		}
	case "service":
		if unit.ServiceID != scopeID {
			return fmt.Errorf("service %d is outside the selected service scope %d", unit.ServiceID, scopeID)
		}
	}
	return nil
}

// generationModelPriceKeys is the ordered set of cost-ledger keys preflight may
// use for one model binding. The DSH runtime is the production billing path and
// records "dsh/<model_key>"; provider/model and bare model are compatibility
// fallbacks so a non-DSH deployment can still price an estimate. Keys are never
// fabricated: a miss on every candidate keeps price_missing.
func generationModelPriceKeys(binding domain.GenerationModelBinding) []string {
	model := strings.TrimSpace(binding.Model)
	if model == "" {
		return nil
	}
	keys := make([]string, 0, 3)
	seen := map[string]struct{}{}
	appendKey := func(key string) {
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	appendKey("dsh/" + model)
	if provider := strings.TrimSpace(binding.Provider); provider != "" {
		appendKey(provider + "/" + model)
	}
	appendKey(model)
	return keys
}

// resolveGenerationPrice walks the candidate billing keys and returns the first
// effective tariff. (nil, nil) means no price is configured for any candidate.
func resolveGenerationPrice(ctx context.Context, port domain.GenerationPlanPricePort, binding domain.GenerationModelBinding) (*domain.GenerationModelPrice, error) {
	for _, key := range generationModelPriceKeys(binding) {
		price, err := port.EffectiveGenerationPrice(ctx, key)
		if err != nil {
			return nil, err
		}
		if price != nil {
			return price, nil
		}
	}
	return nil, nil
}
