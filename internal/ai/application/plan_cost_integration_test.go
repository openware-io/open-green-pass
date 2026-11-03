package application

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	aiinfra "github.com/openware-io/open-green-pass/internal/ai/infra"
	platformdb "github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	trustedinfra "github.com/openware-io/open-green-pass/internal/trusted/infra"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// TestGenerationPlanCostWithRealCostPrice replays preflight against a migrated
// PostgreSQL database using the production billing key shape. It is skipped
// unless GP_TEST_DB_DSN points at a scratch database with migrations 000001-62.
//
// Run manually:
//
//	$env:GP_TEST_DB_DSN='postgres://gp:gp@127.0.0.1:55432/gp?sslmode=disable'
//	go test ./internal/ai/application -run TestGenerationPlanCostWithRealCostPrice -v
func seedPlanCostFixture(t *testing.T, exec func(string, ...any) error, teamID, projectID, repoID, serviceID, batchID, priceID, refID, connectionID int64) {
	t.Helper()
	if err := exec(`INSERT INTO tgt_target(id,team_id,parent_id,level,name,kind,model_binding,status)
		VALUES ($1,$2,NULL,0,'cost-replay-project','api_service','{"Provider":"deepseek","Model":"deepseek-flash"}'::jsonb,'active')`, projectID, teamID); err != nil {
		t.Fatalf("insert target: %v", err)
	}
	if err := exec(`INSERT INTO repo_scm_connection(id,team_id,provider,name,secret_ref,credential_status)
		VALUES ($1,$2,'github','cost-replay-conn','k8s://gp/scm-cost-replay','configured')`, connectionID, teamID); err != nil {
		t.Fatalf("insert scm connection: %v", err)
	}
	if err := exec(`INSERT INTO repo_repo(id,team_id,project_id,kind,url,default_branch,scm_connection_id)
		VALUES ($1,$2,$3,'git','https://github.com/acme/cost-replay','main',$4)`, repoID, teamID, projectID, connectionID); err != nil {
		t.Fatalf("insert repo: %v", err)
	}
	if err := exec(`INSERT INTO tgt_service(id,team_id,project_id,repo_id,name,source_path,kind,status,generation_status,language,framework,profile_status)
		VALUES ($1,$2,$3,$4,'cost-replay-service','services/svc','api_service','active','confirmed','Go','net/http','confirmed')`,
		serviceID, teamID, projectID, repoID); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	if err := exec(`INSERT INTO repo_ref(id,team_id,repo_id,ref_type,name,commit_sha,fetched_at)
		VALUES ($1,$2,$3,'branch','main','deadbeef',now())`, refID, teamID, repoID); err != nil {
		t.Fatalf("insert ref: %v", err)
	}
	if err := exec(`INSERT INTO cost_price(id,team_id,model_key,input_per_1k,output_per_1k,cache_read_per_1k,currency,effective_from,status,created_by)
		VALUES ($1,$2,'dsh/deepseek-flash',0.002,0.008,0,'CNY',now()-interval '1 hour','active',40)`, priceID, teamID); err != nil {
		t.Fatalf("insert cost_price: %v", err)
	}
	if err := exec(`INSERT INTO gen_batch(id,team_id,target_id,repo_id,service_id,source_path,ref_type,branch,state,state_version,idempotency_key,created_by,tokens_in,tokens_out,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,'services/svc','branch','main','approved',1,'cost-replay-batch',40,3000,3500,now()-interval '100 seconds',now())`,
		batchID, teamID, projectID, repoID, serviceID); err != nil {
		t.Fatalf("insert approved batch: %v", err)
	}
}

func TestGenerationPlanCostWithRealCostPrice(t *testing.T) {
	dsn := os.Getenv("GP_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("GP_TEST_DB_DSN is not set; real cost-price replay requires a migrated scratch database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	database := platformdb.NewDB(pool)
	gen, err := id.New(5, nil)
	if err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().UnixNano() % 1_000_000_000_000
	teamID := 7_000_000_000_000 + base
	projectID := 7_100_000_000_000 + base
	repoID := 7_200_000_000_000 + base
	serviceID := 7_300_000_000_000 + base
	batchID := 7_400_000_000_000 + base
	priceID := 7_500_000_000_000 + base
	refID := 7_600_000_000_000 + base
	planID := 7_700_000_000_000 + base
	connectionID := 7_800_000_000_000 + base
	tenantCtx := rls.WithTenant(context.Background(), teamID)

	exec := func(sql string, args ...any) error {
		return database.WithTenant(tenantCtx, func(tx pgx.Tx) error {
			_, err := tx.Exec(tenantCtx, sql, args...)
			return err
		})
	}
	cleanup := func() {
		_ = exec(`DELETE FROM gen_plan WHERE team_id=$1 AND id=$2`, teamID, planID)
		_ = exec(`DELETE FROM gen_batch WHERE team_id=$1 AND id=$2`, teamID, batchID)
		_ = exec(`DELETE FROM cost_price WHERE team_id=$1 AND id=$2`, teamID, priceID)
		_ = exec(`DELETE FROM repo_ref WHERE team_id=$1 AND id=$2`, teamID, refID)
		_ = exec(`DELETE FROM tgt_service WHERE team_id=$1 AND id=$2`, teamID, serviceID)
		_ = exec(`DELETE FROM repo_repo WHERE team_id=$1 AND id=$2`, teamID, repoID)
		_ = exec(`DELETE FROM repo_scm_connection WHERE team_id=$1 AND id=$2`, teamID, connectionID)
		_ = exec(`DELETE FROM tgt_target WHERE team_id=$1 AND id=$2`, teamID, projectID)
	}
	t.Cleanup(cleanup)

	// Real production data shape: model_binding uses capitalized JSON keys and
	// the cost ledger bills the DSH runtime as dsh/<model_key>.
	seedPlanCostFixture(t, exec, teamID, projectID, repoID, serviceID, batchID, priceID, refID, connectionID)

	planStore := aiinfra.NewGenerationPlanStore(database)
	catalog := aiinfra.NewGenerationPlanCatalog(database)
	priceAdapter := aiinfra.NewGenerationPriceAdapter(trustedapp.NewCostPriceService(trustedinfra.NewCostPriceStore(database), nil, gen))
	svc := NewGenerationPlanService(planStore, catalog, fakeRunScopeResolver{}, gen)
	svc.SetPricePort(priceAdapter)

	plan, err := svc.Preflight(tenantCtx, PreflightGenerationPlanCommand{
		ID: planID, TeamID: teamID, ProjectID: projectID, ScopeType: "project", ActorID: 40,
		Units: []domain.GenerationPlanUnitInput{{ServiceID: serviceID, RepoID: repoID, SourcePath: "services/svc", RefType: "branch", RefName: "main"}},
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	assertPlanCostReplay(tenantCtx, t, plan, planStore, teamID, batchID)
}

func assertPlanCostReplay(tenantCtx context.Context, t *testing.T, plan *domain.GenerationPlan, planStore *aiinfra.GenerationPlanStore, teamID, batchID int64) {
	t.Helper()
	if plan.Summary.Services != 1 {
		t.Fatalf("valid services = %d, plan=%+v", plan.Summary.Services, plan.Items)
	}
	for _, risk := range plan.Risks {
		if risk.Code == "price_missing" {
			t.Fatalf("price_missing despite dsh/deepseek-flash tariff: %+v", plan.Risks)
		}
	}
	if plan.Estimate.Unknown {
		t.Fatalf("estimate unexpectedly unknown despite approved batch history")
	}
	if plan.Estimate.TokensIn != 3000 || plan.Estimate.TokensOut != 3500 {
		t.Fatalf("tokens = %d/%d", plan.Estimate.TokensIn, plan.Estimate.TokensOut)
	}
	// 3000/1000*0.002 + 3500/1000*0.008 = 0.006 + 0.028 = 0.034 CNY.
	if plan.Estimate.CostAmount < 0.033 || plan.Estimate.CostAmount > 0.035 {
		t.Fatalf("cost = %v %s, want ≈0.034 CNY", plan.Estimate.CostAmount, plan.Estimate.Currency)
	}
	if plan.Estimate.Currency != "CNY" {
		t.Fatalf("currency = %q", plan.Estimate.Currency)
	}
	// The single-use CAS must succeed once and then report the plan as stale
	// even for a different run id (the P0-2 fix is database-enforced).
	first, err := planStore.Consume(tenantCtx, teamID, plan.ID, batchID, time.Now().UTC())
	if err != nil || !first {
		t.Fatalf("first plan consume = %v, %v", first, err)
	}
	second, err := planStore.Consume(tenantCtx, teamID, plan.ID, batchID+1, time.Now().UTC())
	if err != nil || second {
		t.Fatalf("second plan consume = %v, %v (must be false)", second, err)
	}
}
