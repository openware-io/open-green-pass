package api

import "github.com/openware-io/open-green-pass/internal/execution/domain"

// envRuntimeResponse is the execution-domain HTTP representation of a runtime version.
type envRuntimeResponse struct {
	ID             int64  `json:"id"`
	TeamID         int64  `json:"team_id"`
	TargetID       int64  `json:"target_id"`
	Env            string `json:"env"`
	RunningVersion string `json:"running_version"`
	CheckedAt      string `json:"checked_at"`
}

// envCheckResponse is the execution-domain HTTP representation of a version check.
type envCheckResponse struct {
	ID            int64  `json:"id"`
	TeamID        int64  `json:"team_id"`
	RunID         *int64 `json:"run_id,omitempty"`
	TargetID      int64  `json:"target_id"`
	TargetVersion string `json:"target_version"`
	EnvVersion    string `json:"env_version"`
	Result        string `json:"result"`
	CheckedAt     string `json:"checked_at"`
}

func toEnvRuntimeResponse(runtime *domain.EnvRuntime) envRuntimeResponse {
	return envRuntimeResponse{
		ID: runtime.ID, TeamID: runtime.TeamID, TargetID: runtime.TargetID, Env: runtime.Env,
		RunningVersion: runtime.RunningVersion, CheckedAt: runtime.CheckedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}
}

func toEnvCheckResponse(check *domain.EnvCheck) envCheckResponse {
	return envCheckResponse{
		ID: check.ID, TeamID: check.TeamID, RunID: check.RunID, TargetID: check.TargetID,
		TargetVersion: check.TargetVersion, EnvVersion: check.EnvVersion, Result: string(check.Result),
		CheckedAt: check.CheckedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}
}

func toEnvCheckResponses(checks []*domain.EnvCheck) []envCheckResponse {
	responses := make([]envCheckResponse, 0, len(checks))
	for _, check := range checks {
		responses = append(responses, toEnvCheckResponse(check))
	}
	return responses
}
