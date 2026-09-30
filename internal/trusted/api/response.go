package api

import (
	"time"

	"github.com/openware-io/open-green-pass/internal/trusted/application"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

// gateRuleResponse is the trusted-domain HTTP representation of a gate rule.
type gateRuleResponse struct {
	ID         int64  `json:"id"`
	TeamID     int64  `json:"team_id"`
	TargetID   int64  `json:"target_id"`
	ScenarioID int64  `json:"scenario_id"`
	Rego       string `json:"rego"`
	Version    int    `json:"version"`
	Enabled    bool   `json:"enabled"`
}

// gateResultResponse is the trusted-domain HTTP representation of a gate decision.
type gateResultResponse struct {
	ID        int64          `json:"id"`
	TeamID    int64          `json:"team_id"`
	RunID     int64          `json:"run_id"`
	RuleID    int64          `json:"rule_id"`
	Result    string         `json:"result"`
	Detail    map[string]any `json:"detail,omitempty"`
	DecidedAt time.Time      `json:"decided_at"`
}

// costLineResponse is the trusted-domain HTTP representation of a cost line item.
type costLineResponse struct {
	ID             int64     `json:"id"`
	RequestID      string    `json:"request_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	TeamID         int64     `json:"team_id"`
	TargetID       int64     `json:"target_id"`
	RunID          int64     `json:"run_id"`
	CaseID         int64     `json:"case_id"`
	Category       string    `json:"category"`
	BizPoint       string    `json:"biz_point"`
	Model          string    `json:"model"`
	TokensIn       int64     `json:"tokens_in"`
	TokensOut      int64     `json:"tokens_out"`
	UnitPrice      float64   `json:"unit_price"`
	Amount         float64   `json:"amount"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// costOverviewResponse is the trusted-domain HTTP representation of a cost aggregate.
type costOverviewResponse struct {
	TotalAmount float64            `json:"total_amount"`
	ByCategory  map[string]float64 `json:"by_category"`
	ByPoint     map[string]float64 `json:"by_point"`
}

// reportResponse is the trusted-domain HTTP representation of a generated report.
type reportResponse struct {
	ID        int64          `json:"id"`
	TeamID    int64          `json:"team_id"`
	RunID     int64          `json:"run_id"`
	TargetID  int64          `json:"target_id"`
	Kind      string         `json:"kind"`
	Title     string         `json:"title"`
	Version   string         `json:"version"`
	Branch    string         `json:"branch"`
	Scenario  string         `json:"scenario"`
	Status    string         `json:"status"`
	Summary   map[string]any `json:"summary"`
	Evidence  map[string]any `json:"evidence,omitempty"`
	HTML      string         `json:"html"`
	CreatedAt time.Time      `json:"created_at"`
}

// reportBundleResponse is the trusted-domain HTTP representation of a report bundle.
type reportBundleResponse struct {
	Reports  []reportResponse `json:"reports"`
	Total    int              `json:"total"`
	Pass     int              `json:"pass"`
	Fail     int              `json:"fail"`
	HTML     string           `json:"html,omitempty"`
	Markdown string           `json:"markdown,omitempty"`
}

func toGateRuleResponse(r *domain.GateRule) gateRuleResponse {
	return gateRuleResponse{
		ID: r.ID, TeamID: r.TeamID, TargetID: r.TargetID, ScenarioID: r.ScenarioID,
		Rego: r.Rego, Version: r.Version, Enabled: r.Enabled,
	}
}

func toGateResultResponse(r *domain.GateResult) gateResultResponse {
	return gateResultResponse{
		ID: r.ID, TeamID: r.TeamID, RunID: r.RunID, RuleID: r.RuleID,
		Result: string(r.Result), Detail: r.Detail, DecidedAt: r.DecidedAt,
	}
}

func toGateResultResponses(in []*domain.GateResult) []gateResultResponse {
	out := make([]gateResultResponse, 0, len(in))
	for _, v := range in {
		out = append(out, toGateResultResponse(v))
	}
	return out
}

func toCostLineResponse(c *domain.CostLineItem) costLineResponse {
	return costLineResponse{
		ID: c.ID, RequestID: c.RequestID, IdempotencyKey: c.IdempotencyKey,
		TeamID: c.TeamID, TargetID: c.TargetID, RunID: c.RunID, CaseID: c.CaseID,
		Category: string(c.Category), BizPoint: c.BizPoint, Model: c.Model,
		TokensIn: c.TokensIn, TokensOut: c.TokensOut, UnitPrice: c.UnitPrice,
		Amount: c.Amount, OccurredAt: c.OccurredAt,
	}
}

func toCostLineResponses(in []*domain.CostLineItem) []costLineResponse {
	out := make([]costLineResponse, 0, len(in))
	for _, v := range in {
		out = append(out, toCostLineResponse(v))
	}
	return out
}

func toCostOverviewResponse(v *domain.CostOverview) costOverviewResponse {
	byCategory := make(map[string]float64, len(v.ByCategory))
	for k, amount := range v.ByCategory {
		byCategory[string(k)] = amount
	}
	return costOverviewResponse{TotalAmount: v.TotalAmount, ByCategory: byCategory, ByPoint: v.ByPoint}
}

func toReportResponse(r *domain.Report) reportResponse {
	return reportResponse{
		ID: r.ID, TeamID: r.TeamID, RunID: r.RunID, TargetID: r.TargetID, Kind: r.Kind,
		Title: r.Title, Version: r.Version, Branch: r.Branch, Scenario: r.Scenario,
		Status: r.Status, Summary: r.Summary, Evidence: r.Evidence, HTML: r.HTML, CreatedAt: r.CreatedAt,
	}
}

func toReportBundleResponse(b *application.Bundle) reportBundleResponse {
	out := make([]reportResponse, 0, len(b.Reports))
	for _, r := range b.Reports {
		out = append(out, toReportResponse(r))
	}
	return reportBundleResponse{Reports: out, Total: b.Total, Pass: b.Pass, Fail: b.Fail, HTML: b.HTML, Markdown: b.Markdown}
}
