package api

import "github.com/openware-io/open-green-pass/internal/governance/domain"

// targetResponse 是治理域对外被测对象契约；禁止直接序列化领域聚合。
type targetResponse struct {
	ID       int64  `json:"id"`
	TeamID   int64  `json:"team_id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	ParentID *int64 `json:"parent_id,omitempty"`
	RepoID   *int64 `json:"repo_id,omitempty"`
}

// caseResponse 是治理域对外用例契约；仅暴露用例当前视图。
type caseResponse struct {
	ID             int64  `json:"id"`
	TeamID         int64  `json:"team_id"`
	TargetID       int64  `json:"target_id"`
	Code           string `json:"code"`
	Title          string `json:"title"`
	Kind           string `json:"kind"`
	CurrentVersion int    `json:"current_version"`
	Status         string `json:"status"`
}

func toTargetResponse(target *domain.Target) targetResponse {
	node := target.Node
	return targetResponse{
		ID: node.ID, TeamID: node.TeamID, Type: string(node.Type), Name: node.Name,
		Kind: node.Kind, Status: node.Status, ParentID: node.ParentID, RepoID: node.RepoID,
	}
}

func toTargetResponses(targets []*domain.Target) []targetResponse {
	responses := make([]targetResponse, 0, len(targets))
	for _, target := range targets {
		responses = append(responses, toTargetResponse(target))
	}
	return responses
}

func toCaseResponse(c *domain.Case) caseResponse {
	return caseResponse{
		ID: c.ID, TeamID: c.TeamID, TargetID: c.TargetID, Code: c.Code, Title: c.Title,
		Kind: c.Kind, CurrentVersion: c.CurrentVersion, Status: c.Status,
	}
}

func toCaseResponses(cases []*domain.Case) []caseResponse {
	responses := make([]caseResponse, 0, len(cases))
	for _, c := range cases {
		responses = append(responses, toCaseResponse(c))
	}
	return responses
}
