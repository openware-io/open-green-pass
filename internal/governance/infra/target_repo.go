package infra

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/openware-io/open-green-pass/internal/governance/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// TargetStore 被测对象树仓储实现（RLS 租户由 DB.WithTenant 注入）。
type TargetStore struct {
	db  *DB
	gen *id.Generator
}

// NewTargetStore 创建被测对象树仓储。
func NewTargetStore(db *DB, gen *id.Generator) *TargetStore {
	return &TargetStore{db: db, gen: gen}
}

// nodeTypeLevel 节点类型 -> 层级（0 工程 / 1 服务组 / 2 服务 / 3 模块）。
func nodeTypeLevel(t domain.TargetNodeType) int {
	switch t {
	case domain.NodeProject:
		return 0
	case domain.NodeServiceGroup:
		return 1
	case domain.NodeService:
		return 2
	case domain.NodeModule:
		return 3
	default:
		return 0
	}
}

// SaveTarget 保存被测对象树节点（含可选模型绑定 model_binding JSONB）。
func (s *TargetStore) SaveTarget(ctx context.Context, t *domain.Target) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		id := t.Node.ID
		if id == 0 {
			id = s.gen.Next()
			t.Node.ID = id
		}
		var mb []byte
		if t.ModelBinding != nil {
			b, err := json.Marshal(t.ModelBinding)
			if err != nil {
				return err
			}
			mb = b
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO tgt_target(id, parent_id, level, name, remark, kind, model_binding, status, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			id, t.Node.ParentID, nodeTypeLevel(t.Node.Type), t.Node.Name, t.Node.Remark, t.Node.Kind,
			mb, t.Node.Status, t.Node.OwnerID)
		return err
	})
}

// FindTarget 查询被测对象节点及其绑定仓库/分支（来源溯源）。
func (s *TargetStore) FindTarget(ctx context.Context, teamID, id int64) (*domain.Target, error) {
	var tgt domain.Target
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT n.id, n.parent_id, n.level, n.name, n.remark, n.kind, n.model_binding, n.status, n.created_by
			FROM tgt_target n WHERE n.id = $1 AND n.team_id = $2`,
			id, teamID)
		return scanTarget(row, &tgt)
	})
	if err != nil {
		return nil, err
	}
	return &tgt, nil
}

// ListTargets 列出被测对象树（层级/名称过滤）。
func (s *TargetStore) ListTargets(ctx context.Context, teamID int64, f domain.TargetFilter) ([]*domain.Target, error) {
	where := "n.team_id = $1"
	args := []interface{}{teamID}
	if f.Level != nil {
		args = append(args, *f.Level)
		where += " AND n.level = $" + itoa(len(args))
	}
	if f.Name != "" {
		args = append(args, "%"+f.Name+"%")
		where += " AND n.name ILIKE $" + itoa(len(args))
	}
	var out []*domain.Target
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, parent_id, level, name, remark, kind, model_binding, status, created_by
			FROM tgt_target n WHERE `+where+` ORDER BY level, name`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var tgt domain.Target
			if err := scanNode(rows, &tgt); err != nil {
				return err
			}
			out = append(out, &tgt)
		}
		return rows.Err()
	})
	return out, err
}

// SaveRepo 绑定被测仓库（来源溯源）。
func (s *TargetStore) SaveRepo(ctx context.Context, r *domain.Repo) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		id := r.ID
		if id == 0 {
			id = s.gen.Next()
			r.ID = id
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO repo_repo(id, target_id, kind, url, default_branch, cred_ref, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, r.TargetID, r.Kind, r.URL, r.DefaultBranch, r.CredRef, r.CreatedBy)
		if err != nil {
			return err
		}
		return err
	})
}

// SaveBranch 保存仓库分支/版本（版本校验依据）。
func (s *TargetStore) SaveBranch(ctx context.Context, b *domain.RepoBranch) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		id := b.ID
		if id == 0 {
			id = s.gen.Next()
			b.ID = id
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO repo_branch(id, repo_id, branch, version, head_sha, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (repo_id, branch) DO UPDATE SET version = EXCLUDED.version, head_sha = EXCLUDED.head_sha, updated_at = now()`,
			id, b.RepoID, b.Branch, b.Version, b.HeadSHA, b.CreatedBy)
		return err
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// scanTarget 扫描被测对象节点。仓库改由项目目录接口独立查询。
func scanTarget(row pgx.Row, tgt *domain.Target) error {
	var (
		nodeID, level, createdBy   int64
		parentID                   *int64
		kind, name, remark, status string
		mb                         []byte
	)
	if err := row.Scan(
		&nodeID, &parentID, &level, &name, &remark, &kind, &mb, &status, &createdBy,
	); err != nil {
		return err
	}
	tgt.Node = domain.TargetNode{
		ID: nodeID, Type: levelToNodeType(level), Name: name, Remark: remark, Kind: kind,
		Status: status, OwnerID: createdBy,
	}
	tgt.Node.ParentID = parentID
	if len(mb) > 0 {
		var b domain.ModelBinding
		if err := json.Unmarshal(mb, &b); err == nil {
			tgt.ModelBinding = &b
		}
	}
	return nil
}

// scanNode 扫描单节点（ListTargets 行）。
func scanNode(rows pgx.Rows, tgt *domain.Target) error {
	var (
		nodeID, level, createdBy   int64
		parentID                   *int64
		kind, name, remark, status string
		mb                         []byte
	)
	if err := rows.Scan(&nodeID, &parentID, &level, &name, &remark, &kind, &mb, &status, &createdBy); err != nil {
		return err
	}
	tgt.Node = domain.TargetNode{
		ID: nodeID, Type: levelToNodeType(level), Name: name, Remark: remark, Kind: kind,
		Status: status, OwnerID: createdBy,
	}
	tgt.Node.ParentID = parentID
	if len(mb) > 0 {
		var b domain.ModelBinding
		if err := json.Unmarshal(mb, &b); err == nil {
			tgt.ModelBinding = &b
		}
	}
	return nil
}

// levelToNodeType 层级 -> 节点类型。
func levelToNodeType(level int64) domain.TargetNodeType {
	switch level {
	case 0:
		return domain.NodeProject
	case 1:
		return domain.NodeServiceGroup
	case 2:
		return domain.NodeService
	case 3:
		return domain.NodeModule
	default:
		return domain.NodeProject
	}
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
