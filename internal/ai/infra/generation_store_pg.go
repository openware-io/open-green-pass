package infra

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
	governance "github.com/openware-io/open-green-pass/internal/governance/infra"
)

// GenerationBatchStore persists the generation state machine in gen_batch.
// The application service remains responsible for legal transitions; this
// repository only provides tenant-scoped reads and compare-and-swap writes.
type GenerationBatchStore struct {
	db *governance.DB
}

type GenerationSeed struct {
	ID             int64          `json:"id"`
	BatchID        int64          `json:"batch_id"`
	SourceLocator  string         `json:"source_locator"`
	NormalizedSpec map[string]any `json:"normalized_spec"`
	QualityState   string         `json:"quality_state"`
	ApprovalState  string         `json:"approval_state"`
}

type modelInput struct{ Provider, Model, Protocol, BaseURL, SecretRef string }

// GenerateModelSeed calls the model selected on the target. A batch is never
// marked complete from a local/demo adapter: without a bound and configured
// model this method returns an actionable error.
func (s *GenerationBatchStore) GenerateModelSeed(ctx context.Context, teamID, batchID, seedID int64) error {
	var targetID, repoID int64
	var sourcePath, branch string
	var input modelInput
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT target_id,repo_id,COALESCE(source_path,''),branch FROM gen_batch WHERE team_id=$1 AND id=$2`, teamID, batchID).Scan(&targetID, &repoID, &sourcePath, &branch); err != nil {
			return err
		}
		var binding []byte
		if err := tx.QueryRow(ctx, `SELECT COALESCE(model_binding,'{}'::jsonb) FROM tgt_target WHERE team_id=$1 AND id=$2`, teamID, targetID).Scan(&binding); err != nil {
			return err
		}
		var selected struct{ Provider, Model string }
		if err := json.Unmarshal(binding, &selected); err != nil || strings.TrimSpace(selected.Provider) == "" || strings.TrimSpace(selected.Model) == "" {
			return fmt.Errorf("generation model is not bound to target")
		}
		return tx.QueryRow(ctx, `SELECT provider,model_key,protocol,base_url,COALESCE(secret_ref,'') FROM mdl_model WHERE team_id=$1 AND provider=$2 AND model_key=$3 AND status='approved' AND credential_status IN ('configured','verified')`, teamID, selected.Provider, selected.Model).Scan(&input.Provider, &input.Model, &input.Protocol, &input.BaseURL, &input.SecretRef)
	})
	if err != nil {
		return err
	}
	token, err := resolveModelToken(ctx, input.SecretRef)
	if err != nil {
		return err
	}
	prompt := fmt.Sprintf("You are a software test engineer. Generate one high-value executable test-case candidate for service kind and source path. Return strict JSON with title, kind, preconditions, steps, expected, and source_locator. Service source path: %s; branch: %s; repository id: %d.", sourcePath, branch, repoID)
	text, err := callModel(ctx, input, token, prompt)
	if err != nil {
		return err
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(text), &parsed) != nil {
		parsed = map[string]any{"title": "模型生成候选用例", "kind": "model-generated", "content": text}
	}
	parsed["batch_id"], parsed["model"], parsed["provider"], parsed["source_path"] = batchID, input.Model, input.Provider, sourcePath
	return s.CreateSeed(ctx, teamID, seedID, batchID, "model://"+input.Provider+"/"+input.Model+"?source_path="+sourcePath, parsed)
}

func resolveModelToken(ctx context.Context, ref string) (string, error) {
	if v := strings.TrimSpace(getenv("GP_MODEL_TOKEN")); v != "" {
		return v, nil
	}
	parts := strings.Split(strings.TrimPrefix(ref, "k8s://"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("model credential is not available")
	}
	name, key := parts[1], "token"
	if p := strings.SplitN(name, "#", 2); len(p) == 2 {
		name, key = p[0], p[1]
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("model credential cannot be resolved: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", err
	}
	secret, err := client.CoreV1().Secrets(parts[0]).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if value := secret.Data[key]; len(value) > 0 {
		return string(value), nil
	}
	return "", fmt.Errorf("model credential key %q is missing", key)
}

func callModel(ctx context.Context, input modelInput, token, prompt string) (string, error) {
	if input.Protocol != "api" {
		return "", fmt.Errorf("model protocol %s is not supported by generation worker", input.Protocol)
	}
	payload, _ := json.Marshal(map[string]any{"model": input.Model, "temperature": 0.2, "messages": []map[string]string{{"role": "user", "content": prompt}}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(input.BaseURL, "/")+"/chat/completions", strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("model request failed: %s", strings.TrimSpace(string(body)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("model returned no content")
	}
	return out.Choices[0].Message.Content, nil
}

func getenv(key string) string { return strings.TrimSpace(os.Getenv(key)) }

func (s *GenerationBatchStore) CreateSeed(ctx context.Context, teamID int64, seedID int64, batchID int64, locator string, spec map[string]any) error {
	payload, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	digest := md5.Sum(append([]byte(locator), payload...))
	dedupeHash := hex.EncodeToString(digest[:])
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO gen_seed (id,team_id,batch_id,source_locator,normalized_spec,dedupe_hash) VALUES ($1,$2,$3,$4,$5::jsonb,$6) ON CONFLICT (batch_id,dedupe_hash) DO NOTHING`, seedID, teamID, batchID, locator, payload, dedupeHash)
		return err
	})
}

func (s *GenerationBatchStore) MarkSeedsQuality(ctx context.Context, teamID, batchID int64, state string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE gen_seed SET quality_state=$3 WHERE team_id=$1 AND batch_id=$2`, teamID, batchID, state)
		return err
	})
}

func (s *GenerationBatchStore) CommitSeeds(ctx context.Context, teamID, batchID int64, genNext func() int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var targetID, repoID int64
		var branch string
		if err := tx.QueryRow(ctx, `SELECT target_id,repo_id,branch FROM gen_batch WHERE team_id=$1 AND id=$2`, teamID, batchID).Scan(&targetID, &repoID, &branch); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id,normalized_spec FROM gen_seed WHERE team_id=$1 AND batch_id=$2 AND quality_state='passed'`, teamID, batchID)
		if err != nil {
			return err
		}
		type seedRow struct {
			id  int64
			raw []byte
		}
		seeds := make([]seedRow, 0)
		for rows.Next() {
			var seedID int64
			var raw []byte
			if err := rows.Scan(&seedID, &raw); err != nil {
				rows.Close()
				return err
			}
			seeds = append(seeds, seedRow{id: seedID, raw: raw})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, seed := range seeds {
			seedID, raw := seed.id, seed.raw
			var spec map[string]any
			if err := json.Unmarshal(raw, &spec); err != nil {
				return err
			}
			code := fmt.Sprintf("GEN-%d-%d", batchID, seedID)
			title, _ := spec["title"].(string)
			if title == "" {
				title = fmt.Sprintf("生成候选种子 %d", seedID)
			}
			caseID, versionID := genNext(), genNext()
			if _, err := tx.Exec(ctx, `INSERT INTO cas_case (id,team_id,target_id,code,title,kind,created_by) VALUES ($1,$2,$3,$4,$5,'api',1) ON CONFLICT (team_id,target_id,code) DO NOTHING`, caseID, teamID, targetID, code, title); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO cas_version (id,team_id,case_id,version,change_type,source_repo_id,source_branch,script_json,approved_at) SELECT $1,$2,id,1,'added',$3,$4,$5,now() FROM cas_case WHERE team_id=$2 AND target_id=$6 AND code=$7 ON CONFLICT DO NOTHING`, versionID, teamID, repoID, branch, raw, targetID, code); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE gen_seed SET approval_state='approved' WHERE team_id=$1 AND batch_id=$2 AND id=$3`, teamID, batchID, seedID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GenerationBatchStore) ListSeeds(ctx context.Context, teamID, batchID int64) ([]GenerationSeed, error) {
	out := make([]GenerationSeed, 0)
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,batch_id,source_locator,normalized_spec,quality_state,approval_state FROM gen_seed WHERE team_id=$1 AND batch_id=$2 ORDER BY id`, teamID, batchID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var seed GenerationSeed
			var normalized []byte
			if err := rows.Scan(&seed.ID, &seed.BatchID, &seed.SourceLocator, &normalized, &seed.QualityState, &seed.ApprovalState); err != nil {
				return err
			}
			if len(normalized) > 0 {
				if err := json.Unmarshal(normalized, &seed.NormalizedSpec); err != nil {
					return err
				}
			}
			out = append(out, seed)
		}
		return rows.Err()
	})
	return out, err
}

func NewGenerationBatchStore(db *governance.DB) *GenerationBatchStore {
	return &GenerationBatchStore{db: db}
}

func (s *GenerationBatchStore) Create(ctx context.Context, b *domain.GenerationBatch) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO gen_batch
            (id,team_id,target_id,repo_id,service_id,source_path,branch,requested_version,state,state_version,workflow_id,idempotency_key,parent_batch_id,created_by)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			b.ID, b.TeamID, b.TargetID, b.RepoID, b.ServiceID, nullString(b.SourcePath), b.Branch, nullString(b.RequestedVersion), string(b.State), b.StateVersion,
			b.WorkflowIdentity(), b.IdempotencyKey, b.ParentBatchID, b.CreatedBy)
		return err
	})
}

func (s *GenerationBatchStore) Find(ctx context.Context, teamID, id int64) (*domain.GenerationBatch, error) {
	var b domain.GenerationBatch
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return scanBatch(tx.QueryRow(ctx, `SELECT id,team_id,target_id,repo_id,service_id,COALESCE(source_path,''),branch,COALESCE(requested_version,''),COALESCE(head_sha,''),state,state_version,COALESCE(workflow_id,''),idempotency_key,parent_batch_id,created_by,COALESCE(failure_summary,''),created_at,updated_at FROM gen_batch WHERE team_id=$1 AND id=$2`, teamID, id), &b)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGenerationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *GenerationBatchStore) FindByIdempotencyKey(ctx context.Context, teamID int64, key string) (*domain.GenerationBatch, error) {
	var b domain.GenerationBatch
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return scanBatch(tx.QueryRow(ctx, `SELECT id,team_id,target_id,repo_id,service_id,COALESCE(source_path,''),branch,COALESCE(requested_version,''),COALESCE(head_sha,''),state,state_version,COALESCE(workflow_id,''),idempotency_key,parent_batch_id,created_by,COALESCE(failure_summary,''),created_at,updated_at FROM gen_batch WHERE team_id=$1 AND idempotency_key=$2`, teamID, key), &b)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGenerationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *GenerationBatchStore) CompareAndSwap(ctx context.Context, b *domain.GenerationBatch, expected int64) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `UPDATE gen_batch SET state=$3,state_version=$4,updated_at=$5,updated_by=$6 WHERE team_id=$1 AND id=$2 AND state_version=$7`, b.TeamID, b.ID, string(b.State), b.StateVersion, b.UpdatedAt, b.CreatedBy, expected)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return domain.ErrGenerationStateConflict
		}
		return nil
	})
}

func scanBatch(row pgx.Row, b *domain.GenerationBatch) error {
	var state string
	if err := row.Scan(&b.ID, &b.TeamID, &b.TargetID, &b.RepoID, &b.ServiceID, &b.SourcePath, &b.Branch, &b.RequestedVersion, &b.HeadSHA, &state, &b.StateVersion, &b.WorkflowID, &b.IdempotencyKey, &b.ParentBatchID, &b.CreatedBy, &b.FailureSummary, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return err
	}
	b.State = domain.GenerationBatchState(state)
	return nil
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
