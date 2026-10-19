package application

import (
	"context"
	"errors"
	"testing"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/ai/infra"
)

type testGenerationScopePort struct {
	repoID int64
	mutate func(*domain.GenerationScope)
	err    error
}

func (p testGenerationScopePort) ReadGenerationScope(_ context.Context, teamID, targetID, serviceID int64) (*domain.GenerationScope, error) {
	if p.err != nil {
		return nil, p.err
	}
	// Execution commands are discovery metadata, not user-supplied generation
	// prerequisites. Keep them empty in the baseline scope to prove that a
	// service with no repository test script can still generate HTTP candidates.
	scope := &domain.GenerationScope{TeamID: teamID, TargetID: targetID, ServiceID: serviceID, RepoID: p.repoID, SourcePath: ".", Status: "active", GenerationStatus: "confirmed", ProfileStatus: "confirmed", Language: "Go", Framework: "net/http"}
	if p.mutate != nil {
		p.mutate(scope)
	}
	return scope, nil
}

func generationTestServiceID() *int64 { id := int64(50); return &id }

type generationStartProbe struct{ starts int }

func (p *generationStartProbe) Start(context.Context, int64, int64) error {
	p.starts++
	return nil
}

func TestGenerationCreateRejectsUnconfirmedOrSpoofedScopeBeforePersistence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*domain.GenerationScope)
		command func(*CreateGenerationBatchCommand)
		err     error
		noPort  bool
	}{
		{name: "missing scope reader", noPort: true},
		{name: "missing service", command: func(c *CreateGenerationBatchCommand) { c.ServiceID = nil }},
		{name: "missing catalog record", err: domain.ErrGenerationNotFound},
		{name: "foreign tenant", mutate: func(s *domain.GenerationScope) { s.TeamID++ }},
		{name: "foreign project", mutate: func(s *domain.GenerationScope) { s.TargetID++ }},
		{name: "foreign service", mutate: func(s *domain.GenerationScope) { s.ServiceID++ }},
		{name: "foreign repo", mutate: func(s *domain.GenerationScope) { s.RepoID++ }},
		{name: "disabled service", mutate: func(s *domain.GenerationScope) { s.Status = "disabled" }},
		{name: "generation unconfirmed", mutate: func(s *domain.GenerationScope) { s.GenerationStatus = "pending" }},
		{name: "profile unconfirmed", mutate: func(s *domain.GenerationScope) { s.ProfileStatus = "suggested" }},
		{name: "profile incomplete", mutate: func(s *domain.GenerationScope) { s.Framework = " " }},
		{name: "spoofed source", mutate: func(s *domain.GenerationScope) { s.SourcePath = "api" }, command: func(c *CreateGenerationBatchCommand) { c.SourcePath = "other" }},
		{name: "broadened source", mutate: func(s *domain.GenerationScope) { s.SourcePath = "api" }, command: func(c *CreateGenerationBatchCommand) { c.SourcePath = "." }},
		{name: "traversal source", command: func(c *CreateGenerationBatchCommand) { c.SourcePath = "../other" }},
		{name: "unsafe catalog source", mutate: func(s *domain.GenerationScope) { s.SourcePath = "../other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := infra.NewMemoryGenerationBatchRepository()
			svc := NewGenerationBatchService(repo)
			workflow := &generationStartProbe{}
			svc.SetWorkflowPort(workflow)
			if !tc.noPort {
				svc.SetScopePort(testGenerationScopePort{repoID: 30, mutate: tc.mutate, err: tc.err})
			}
			cmd := CreateGenerationBatchCommand{ID: 1, TeamID: 10, TargetID: 20, RepoID: 30, ServiceID: generationTestServiceID(), Branch: "main", IdempotencyKey: "scope", ActorID: 40}
			if tc.command != nil {
				tc.command(&cmd)
			}
			if _, created, err := svc.Create(context.Background(), cmd); err == nil || created {
				t.Fatalf("unsafe create succeeded: created=%t err=%v", created, err)
			}
			if _, err := repo.FindByIdempotencyKey(context.Background(), 10, "scope"); !errors.Is(err, domain.ErrGenerationNotFound) {
				t.Fatalf("rejected request persisted: %v", err)
			}
			if workflow.starts != 0 {
				t.Fatalf("rejected request started %d workflows", workflow.starts)
			}
		})
	}
}

func TestGenerationCreatePinsCanonicalCatalogPathAndRejectsIdempotencyScopeChange(t *testing.T) {
	for _, requested := range []string{"", "services/api", "./services/api/", `services\api`} {
		t.Run(requested, func(t *testing.T) {
			repo := infra.NewMemoryGenerationBatchRepository()
			svc := NewGenerationBatchService(repo)
			svc.SetScopePort(testGenerationScopePort{repoID: 30, mutate: func(s *domain.GenerationScope) { s.SourcePath = "./services/api/" }})
			workflow := &generationStartProbe{}
			svc.SetWorkflowPort(workflow)
			cmd := CreateGenerationBatchCommand{ID: 1, TeamID: 10, TargetID: 20, RepoID: 30, ServiceID: generationTestServiceID(), SourcePath: requested, Branch: "main", IdempotencyKey: "scope", ActorID: 40}
			batch, created, err := svc.Create(context.Background(), cmd)
			if err != nil || !created || batch.SourcePath != "services/api" || workflow.starts != 1 {
				t.Fatalf("canonical create: batch=%#v created=%t err=%v", batch, created, err)
			}
			cmd.Branch = "other"
			if _, created, err := svc.Create(context.Background(), cmd); err == nil || created || workflow.starts != 1 {
				t.Fatalf("changed idempotent request must not restart workflow: created=%t starts=%d err=%v", created, workflow.starts, err)
			}
			cmd.Branch = "main"
			svc.SetScopePort(testGenerationScopePort{repoID: 30, mutate: func(s *domain.GenerationScope) { s.GenerationStatus = "pending" }})
			if _, created, err := svc.Create(context.Background(), cmd); err == nil || created || workflow.starts != 1 {
				t.Fatalf("revoked scope must not restart enqueued workflow: created=%t starts=%d err=%v", created, workflow.starts, err)
			}
		})
	}
}
