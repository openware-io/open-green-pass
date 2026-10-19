package infra

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/openware-io/open-green-pass/internal/assistant/application"
	"github.com/openware-io/open-green-pass/internal/platform/db"
)

type Store struct{ db *db.DB }

func NewStore(db *db.DB) *Store { return &Store{db: db} }
func (s *Store) CreateSession(ctx context.Context, v application.Session, subject string) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO assistant_session(id,team_id,subject,page_context,created_at,updated_at) VALUES($1,0,$2,$3,$4,$5)`, v.ID, subject, v.Page, v.CreatedAt, v.UpdatedAt)
		return e
	})
}
func (s *Store) Session(ctx context.Context, id int64) (v application.Session, err error) {
	err = s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id,subject,page_context,created_at,updated_at FROM assistant_session WHERE id=$1`, id).Scan(&v.ID, &v.Subject, &v.Page, &v.CreatedAt, &v.UpdatedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		err = application.ErrNotFound
	}
	return
}
func (s *Store) AddTurns(ctx context.Context, turns []application.Turn) error {
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		for _, v := range turns {
			if _, err := tx.Exec(ctx, `INSERT INTO assistant_turn(id,team_id,session_id,role,content,created_at) VALUES($1,0,$2,$3,$4,$5)`, v.ID, v.SessionID, v.Role, v.Text, v.CreatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Store) Turns(ctx context.Context, sessionID int64) (out []application.Turn, err error) {
	out = []application.Turn{}
	err = s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT id,session_id,role,content,created_at FROM assistant_turn WHERE session_id=$1 ORDER BY created_at,id`, sessionID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var v application.Turn
			if e := rows.Scan(&v.ID, &v.SessionID, &v.Role, &v.Text, &v.CreatedAt); e != nil {
				return e
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return
}
func (s *Store) CreateArtifact(ctx context.Context, v application.Artifact) error {
	evidence, _ := json.Marshal(v.Evidence)
	return s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO assistant_artifact(id,team_id,kind,priority,title,summary,recommendation,evidence,status,created_at) VALUES($1,0,$2,$3,$4,$5,$6,$7,$8,$9)`, v.ID, v.Kind, v.Priority, v.Title, v.Summary, v.Recommendation, evidence, v.Status, v.CreatedAt)
		return e
	})
}
func (s *Store) Artifacts(ctx context.Context) (out []application.Artifact, err error) {
	out = []application.Artifact{}
	err = s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT id,kind,priority,title,summary,recommendation,evidence,status,created_at FROM assistant_artifact ORDER BY created_at DESC LIMIT 100`)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var v application.Artifact
			var raw []byte
			if e := rows.Scan(&v.ID, &v.Kind, &v.Priority, &v.Title, &v.Summary, &v.Recommendation, &raw, &v.Status, &v.CreatedAt); e != nil {
				return e
			}
			_ = json.Unmarshal(raw, &v.Evidence)
			out = append(out, v)
		}
		return rows.Err()
	})
	return
}
func (s *Store) CreateAction(ctx context.Context, sessionID int64, v application.UIAction, status string) (out application.ActionRecord, err error) {
	err = s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		values, _ := json.Marshal(v.Values)
		return tx.QueryRow(ctx, `INSERT INTO assistant_action(id,team_id,session_id,action_kind,route,target,values,status) VALUES($1,0,$2,$3,$4,$5,$6,$7) RETURNING id,action_kind,status,created_at`, v.ID, sessionID, v.Kind, v.Route, v.Target, values, status).Scan(&out.ID, &out.Action, &out.Status, &out.CreatedAt)
	})
	return
}
func (s *Store) Actions(ctx context.Context, sessionID int64) (out []application.ActionRecord, err error) {
	out = []application.ActionRecord{}
	err = s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT id,action_kind,status,created_at FROM assistant_action WHERE session_id=$1 ORDER BY created_at,id`, sessionID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var v application.ActionRecord
			if e := rows.Scan(&v.ID, &v.Action, &v.Status, &v.CreatedAt); e != nil {
				return e
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return
}

func (s *Store) ActionBelongs(ctx context.Context, sessionID, actionID int64) (ok bool, err error) {
	err = s.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assistant_action WHERE session_id=$1 AND id=$2)`, sessionID, actionID).Scan(&ok)
	})
	return
}

var _ application.Repository = (*Store)(nil)
