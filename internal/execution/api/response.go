package api

import (
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

// runResponse 是执行域对外运行契约；领域聚合不负责 JSON 字段命名。
type runResponse struct {
	ID            int64      `json:"id"`
	TeamID        int64      `json:"team_id"`
	ScenarioID    int64      `json:"scenario_id"`
	TargetID      int64      `json:"target_id"`
	Env           string     `json:"env"`
	TargetVersion string     `json:"target_version"`
	TargetBranch  string     `json:"target_branch"`
	EnvVersion    string     `json:"env_version"`
	EnvCheckID    *int64     `json:"env_check_id,omitempty"`
	RunMode       string     `json:"run_mode"`
	State         string     `json:"state"`
	SelectedCases []int64    `json:"selected_cases"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
}

type caseResultResponse struct {
	ID          int64               `json:"id"`
	TeamID      int64               `json:"team_id"`
	RunID       int64               `json:"run_id"`
	CaseID      int64               `json:"case_id"`
	CaseVersion int                 `json:"case_version"`
	Status      string              `json:"status"`
	ResultText  string              `json:"result_text"`
	Evidence    *domain.EvidenceRef `json:"evidence,omitempty"`
	AttemptSeq  int                 `json:"attempt_seq"`
	StartedAt   time.Time           `json:"started_at"`
	EndedAt     *time.Time          `json:"ended_at,omitempty"`
}

func toRunResponse(run *domain.Run) runResponse {
	return runResponse{
		ID: run.ID, TeamID: run.TeamID, ScenarioID: run.ScenarioID, TargetID: run.TargetID,
		Env: run.Env, TargetVersion: run.TargetVersion, TargetBranch: run.TargetBranch,
		EnvVersion: run.EnvVersion, EnvCheckID: run.EnvCheckID, RunMode: run.RunMode,
		State: string(run.State), SelectedCases: run.SelectedCases, StartedAt: run.StartedAt, EndedAt: run.EndedAt,
	}
}

func toCaseResultResponse(result *domain.CaseResult) caseResultResponse {
	return caseResultResponse{
		ID: result.ID, TeamID: result.TeamID, RunID: result.RunID, CaseID: result.CaseID,
		CaseVersion: result.CaseVersion, Status: string(result.Status), ResultText: result.ResultText,
		Evidence: result.Evidence, AttemptSeq: result.AttemptSeq, StartedAt: result.StartedAt, EndedAt: result.EndedAt,
	}
}

func toCaseResultResponses(results []*domain.CaseResult) []caseResultResponse {
	responses := make([]caseResultResponse, 0, len(results))
	for _, result := range results {
		responses = append(responses, toCaseResultResponse(result))
	}
	return responses
}
