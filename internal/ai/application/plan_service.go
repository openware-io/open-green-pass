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
//
//nolint:gocyclo // Preflight aggregates all fail-closed validation branches for one plan.
func (s *GenerationPlanService) Preflight(ctx context.Context, cmd PreflightGenerationPlanCommand) (*domain.GenerationPlan, error) {
	if s == nil || s.plans == nil {
		return nil, errors.New("generation plan repository is required")
	}
	if s.catalog == nil || s.scope == nil || s.gen == nil {
		return nil, errors.New("generation plan dependencies are not configured")
	}
	if cmd.TeamID <= 0 || cmd.ProjectID <= 0 || cmd.ActorID <= 0 {
		return nil, fmt.Errorf("%w: tenant, project and actor are required", ErrGenerationPlanValidation)
	}
	if cmd.ID <= 0 {
		cmd.ID = s.gen.Next()
	}
	if !domain.ValidGenerationScopeType(cmd.ScopeType) {
		return nil, fmt.Errorf("%w: scope must be project, repo or service", ErrGenerationPlanValidation)
	}
	if cmd.ScopeType != "project" && cmd.ScopeID <= 0 {
		return nil, fmt.Errorf("%w: repo and service scopes require scope_id", ErrGenerationPlanValidation)
	}
	if cmd.ScopeType == "project" && cmd.ScopeID != 0 && cmd.ScopeID != cmd.ProjectID {
		return nil, fmt.Errorf("%w: project scope_id must be 0 or the project id", ErrGenerationPlanValidation)
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

	items := make([]domain.GenerationPlanItem, 0, len(units))
	risks := make([]domain.GenerationPlanRisk, 0)
	riskKeys := map[string]struct{}{}
	addRisk := func(code, message, severity string) {
		if _, exists := riskKeys[code]; exists {
			return
		}
		riskKeys[code] = struct{}{}
		risks = append(risks, domain.GenerationPlanRisk{Code: code, Message: message, Severity: severity})
	}
	addRisk("review_required", "生成后需人工审核，审核通过才会写入用例库", "info")

	seen := map[int64]struct{}{}
	var estimate domain.GenerationPlanEstimate
	knownUnits := 0
	tokensIn, tokensOut := int64(0), int64(0)
	durationSeconds := 0
	candidatesMin, candidatesMax := 0, 0
	repos := map[int64]struct{}{}
	validServices := 0
	for _, unit := range units {
		if err := validateUnitScopeOwnership(cmd.ScopeType, cmd.ScopeID, unit); err != nil {
			items = append(items, domain.GenerationPlanItem{
				ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
				RefType: unit.RefType, RefName: unit.RefName, Valid: false,
				Error: "scope_mismatch: " + err.Error(),
			})
			continue
		}
		if _, exists := seen[unit.ServiceID]; exists {
			items = append(items, domain.GenerationPlanItem{
				ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
				RefType: unit.RefType, RefName: unit.RefName, Valid: false,
				Error: "duplicate_service: 同一服务在计划中重复出现",
			})
			continue
		}
		seen[unit.ServiceID] = struct{}{}
		item, itemRisks, usage := s.evaluateUnit(ctx, cmd, unit)
		items = append(items, item)
		for _, risk := range itemRisks {
			addRisk(risk.Code, risk.Message, risk.Severity)
		}
		if !item.Valid {
			continue
		}
		validServices++
		repos[item.RepoID] = struct{}{}
		if usage != nil && usage.SampleCount > 0 {
			knownUnits++
			tokensIn += usage.TokensIn
			tokensOut += usage.TokensOut
			durationSeconds += usage.DurationSeconds
			candidatesMin += usage.CandidatesMin
			candidatesMax += usage.CandidatesMax
		} else {
			candidatesMin += 3
			candidatesMax += 8
		}
	}
	if knownUnits == 0 {
		estimate.Unknown = true
	} else {
		estimate.TokensIn = tokensIn
		estimate.TokensOut = tokensOut
		estimate.DurationSeconds = durationSeconds
		if knownUnits < validServices {
			addRisk("estimate_partial", "部分服务没有历史 approved 批次，估算只覆盖有历史数据的服务", "warning")
		}
	}
	currency := "CNY"
	if s.price != nil {
		binding, err := s.catalog.ReadGenerationModel(ctx, cmd.TeamID, cmd.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("read generation model: %w", err)
		}
		price, err := resolveGenerationPrice(ctx, s.price, binding)
		if err != nil {
			return nil, fmt.Errorf("read generation price: %w", err)
		}
		if price == nil {
			addRisk("price_missing", "项目未配置可用模型单价，预估成本未知", "warning")
		} else if estimate.TokensIn > 0 || estimate.TokensOut > 0 {
			estimate.CostAmount = float64(estimate.TokensIn)/1000*price.InputUnitPrice + float64(estimate.TokensOut)/1000*price.OutputUnitPrice
			if strings.TrimSpace(price.Currency) != "" {
				currency = price.Currency
			}
		}
	} else {
		addRisk("price_missing", "成本估算未配置，预估成本未知", "warning")
	}
	estimate.Currency = currency

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
		Items: items,
		Summary: domain.GenerationPlanSummary{
			Services: validServices, Repos: len(repos), Batches: validServices,
			CandidatesMin: candidatesMin, CandidatesMax: candidatesMax,
		},
		Estimate:  estimate,
		Risks:     risks,
		ExpiresAt: s.now().Add(domain.DefaultGenerationPlanTTL),
		CreatedBy: cmd.ActorID,
		CreatedAt: s.now(),
		UpdatedAt: s.now(),
	}
	plan.PlanHash = domain.CanonicalGenerationPlanHash(plan.ProjectID, plan.ScopeType, plan.ScopeID, plan.Items)

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
		s.log.Info("generation plan created", "team_id", cmd.TeamID, "plan_id", plan.ID, "valid_services", validServices, "request_id", cmd.RequestID)
	}
	return plan, nil
}

// evaluateUnit validates one service/reference pair. A non-nil usage means the
// estimate has historical data for this unit.
//
//nolint:gocyclo // Unit validation is a explicit validation matrix with fail-closed reasons.
func (s *GenerationPlanService) evaluateUnit(ctx context.Context, cmd PreflightGenerationPlanCommand, unit domain.GenerationPlanUnitInput) (domain.GenerationPlanItem, []domain.GenerationPlanRisk, *domain.GenerationUsageSample) {
	item := domain.GenerationPlanItem{
		ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
		RefType: unit.RefType, RefName: unit.RefName,
	}
	invalid := func(code, message string) (domain.GenerationPlanItem, []domain.GenerationPlanRisk, *domain.GenerationUsageSample) {
		item.Valid = false
		item.Error = code + ": " + message
		return item, nil, nil
	}
	if unit.ServiceID <= 0 || unit.RepoID <= 0 || strings.TrimSpace(unit.RefName) == "" {
		return invalid("validation_failed", "服务、仓库和代码引用不能为空")
	}
	if unit.RefType != "branch" && unit.RefType != "tag" {
		return invalid("validation_failed", "代码引用类型必须是 branch 或 tag")
	}
	scope, err := s.catalog.ReadGenerationScope(ctx, cmd.TeamID, cmd.ProjectID, unit.ServiceID)
	if err != nil || scope == nil {
		return invalid("service_not_found", "当前项目中不存在该目录服务")
	}
	if scope.TeamID != cmd.TeamID || scope.TargetID != cmd.ProjectID || scope.ServiceID != unit.ServiceID {
		return invalid("service_not_found", "目录服务不属于所选项目")
	}
	if scope.RepoID != unit.RepoID {
		return invalid("service_repo_mismatch", "目录服务不属于所选仓库")
	}
	if scope.Status != "active" || scope.GenerationStatus != "confirmed" || scope.ProfileStatus != "confirmed" {
		return invalid("service_not_confirmed", "请先启用目录服务并确认技术画像与用例生成范围")
	}
	if strings.TrimSpace(scope.Language) == "" || strings.TrimSpace(scope.Framework) == "" {
		return invalid("profile_incomplete", "目录服务的技术画像不完整，请补充语言和框架")
	}
	canonical, pathErr := generationSourcePath(scope.SourcePath)
	if pathErr != nil {
		return invalid("invalid_source_path", "目录服务的源码目录无效，请先修正目录登记")
	}
	requestedPath := unit.SourcePath
	if strings.TrimSpace(requestedPath) == "" {
		requestedPath = scope.SourcePath
	}
	if requested, requestedErr := generationSourcePath(requestedPath); requestedErr != nil || requested != canonical {
		return invalid("invalid_source_path", "请求的源码目录与已确认的目录服务不一致")
	}
	item.SourcePath = scope.SourcePath

	readiness, err := s.catalog.ReadRepositoryReadiness(ctx, cmd.TeamID, cmd.ProjectID, unit.RepoID)
	if errors.Is(err, domain.ErrGenerationRepositoryNotFound) || (err == nil && readiness == nil) {
		return invalid("repo_not_found", "仓库不存在或已删除")
	}
	if err != nil {
		return invalid("repo_unreadable", "仓库读取失败")
	}
	item.RepoURL = readiness.RepoURL
	if !readiness.SCMConfigured {
		return invalid("scm_not_verified", "仓库未绑定可用 SCM 连接，请先配置读取连接")
	}

	var risks []domain.GenerationPlanRisk
	ref, err := s.catalog.ReadRepositoryRef(ctx, cmd.TeamID, unit.RepoID, unit.RefType, unit.RefName)
	if errors.Is(err, domain.ErrGenerationRepositoryRefNotFound) || (err == nil && ref == nil) {
		return invalid("ref_not_found", "代码引用未同步或已失效，请先刷新分支和 Tag")
	}
	if err != nil {
		return invalid("refs_unavailable", "代码引用读取失败")
	}
	item.CommitSHA = ref.CommitSHA
	if !ref.FetchedAt.IsZero() && s.now().Sub(ref.FetchedAt) > 24*time.Hour {
		risks = append(risks, domain.GenerationPlanRisk{Code: "refs_stale", Message: "代码引用缓存超过 24 小时未刷新，建议先刷新引用", Severity: "warning"})
	}

	if _, err := s.catalog.FindLatestApprovedBaseline(ctx, cmd.TeamID, cmd.ProjectID, unit.RepoID, unit.ServiceID); err != nil {
		return invalid("baseline_unavailable", "系统基线读取失败")
	}

	usage, err := s.catalog.ReadServiceUsage(ctx, cmd.TeamID, cmd.ProjectID, unit.RepoID, unit.ServiceID)
	if err != nil {
		return invalid("estimate_unavailable", "历史用量读取失败")
	}
	if usage == nil || usage.SampleCount == 0 {
		fallback, fallbackErr := s.catalog.ReadFallbackUsage(ctx, cmd.TeamID, cmd.ProjectID)
		if fallbackErr != nil {
			return invalid("estimate_unavailable", "历史用量读取失败")
		}
		if fallback != nil && fallback.SampleCount > 0 {
			usage = fallback
			risks = append(risks, domain.GenerationPlanRisk{Code: "estimate_fallback", Message: "该服务无历史 approved 批次，估算使用项目历史均值", Severity: "warning"})
		} else {
			usage = nil
		}
	}
	if usage != nil && usage.SampleCount > 0 {
		if usage.CandidatesMin <= 0 {
			usage.CandidatesMin = 3
		}
		if usage.CandidatesMax < usage.CandidatesMin {
			usage.CandidatesMax = usage.CandidatesMin
		}
	}
	item.Valid = true
	return item, risks, usage
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
