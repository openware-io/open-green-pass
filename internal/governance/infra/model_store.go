package infra

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openware-io/open-green-pass/internal/governance/domain"
)

var ErrModelNotFound = errors.New("model not found")
var ErrPriceNotFound = errors.New("price snapshot not found")

type ModelStore struct {
	db *DB
}

func NewModelStore(db *DB) *ModelStore { return &ModelStore{db: db} }

func (s *ModelStore) CreateModel(ctx context.Context, m *domain.ModelConfig) error {
	if err := m.Validate(); err != nil {
		return err
	}
	caps, err := json.Marshal(m.Capabilities)
	if err != nil {
		return err
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO mdl_model
			(id, team_id, name, provider, model_key, capabilities, status, secret_ref, endpoint_ref,
			 default_timeout_ms, max_tokens_in, max_tokens_out, revision, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			m.ID, m.TeamID, m.Name, m.Provider, m.ModelKey, caps, m.Status, m.SecretRef,
			m.EndpointRef, m.DefaultTimeout, m.MaxTokensIn, m.MaxTokensOut, m.Revision, m.CreatedBy)
		return err
	})
}

func (s *ModelStore) FindModel(ctx context.Context, teamID, id int64) (*domain.ModelConfig, error) {
	var m domain.ModelConfig
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		var caps []byte
		err := tx.QueryRow(ctx, `SELECT id,team_id,name,provider,model_key,capabilities,status,
			secret_ref,endpoint_ref,default_timeout_ms,max_tokens_in,max_tokens_out,revision,created_by
			FROM mdl_model WHERE team_id=$1 AND id=$2`, teamID, id).Scan(
			&m.ID, &m.TeamID, &m.Name, &m.Provider, &m.ModelKey, &caps, &m.Status, &m.SecretRef, &m.EndpointRef,
			&m.DefaultTimeout, &m.MaxTokensIn, &m.MaxTokensOut, &m.Revision, &m.CreatedBy)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrModelNotFound
		}
		if err != nil {
			return err
		}
		return json.Unmarshal(caps, &m.Capabilities)
	})
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *ModelStore) ListModels(ctx context.Context, teamID int64) ([]*domain.ModelConfig, error) {
	var out []*domain.ModelConfig
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,team_id,name,provider,model_key,capabilities,status,
			secret_ref,endpoint_ref,default_timeout_ms,max_tokens_in,max_tokens_out,revision,created_by
			FROM mdl_model WHERE team_id=$1 ORDER BY name,id`, teamID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m domain.ModelConfig
			var caps []byte
			if err := rows.Scan(&m.ID, &m.TeamID, &m.Name, &m.Provider, &m.ModelKey, &caps, &m.Status, &m.SecretRef, &m.EndpointRef,
				&m.DefaultTimeout, &m.MaxTokensIn, &m.MaxTokensOut, &m.Revision, &m.CreatedBy); err != nil {
				return err
			}
			if err := json.Unmarshal(caps, &m.Capabilities); err != nil {
				return err
			}
			out = append(out, &m)
		}
		return rows.Err()
	})
	return out, err
}

func (s *ModelStore) AddPrice(ctx context.Context, p *domain.PriceSnapshot) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO cost_price
			(id,team_id,model_id,currency,input_per_1k,output_per_1k,effective_from,effective_to,status,approved_by,revision)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, p.ID, p.TeamID, p.ModelID, p.Currency,
			p.InputPer1K, p.OutputPer1K, p.EffectiveFrom, p.EffectiveTo, p.Status, p.ApprovedBy, p.Revision)
		return err
	})
}

func (s *ModelStore) ResolvePrice(ctx context.Context, teamID, modelID int64, at time.Time) (*domain.PriceSnapshot, error) {
	var p domain.PriceSnapshot
	err := s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT id,model_id,currency,input_per_1k,output_per_1k,effective_from,effective_to,status,approved_by,revision
			FROM cost_price WHERE team_id=$1 AND model_id=$2 AND status='active'
			AND effective_from <= $3 AND (effective_to IS NULL OR effective_to > $3)
			ORDER BY effective_from DESC, revision DESC LIMIT 1`, teamID, modelID, at).Scan(
			&p.ID, &p.ModelID, &p.Currency, &p.InputPer1K, &p.OutputPer1K, &p.EffectiveFrom, &p.EffectiveTo, &p.Status, &p.ApprovedBy, &p.Revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPriceNotFound
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

var _ domain.ModelGovernanceRepository = (*ModelStore)(nil)
