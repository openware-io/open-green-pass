//go:build integration

package infra

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Temporary table fixtures exercise the real PostgreSQL statement without
// touching tenant catalog rows or requiring a particular migration baseline.
func TestServiceGenerationConfirmation(t *testing.T) {
	dsn := os.Getenv("GP_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://gp:gp@127.0.0.1:5433/gp?sslmode=disable"
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `CREATE TEMPORARY TABLE tgt_service (
		id bigint, project_id bigint, kind text, generation_status text,
		language text, framework text, test_tool text, test_command text, profile_status text
	) ON COMMIT DROP;
	INSERT INTO tgt_service VALUES
		(1, 9, 'api_service', 'pending', 'Java', 'Spring Boot', 'JUnit', 'mvn test', 'suggested'),
		(2, 9, 'web_app', 'pending', 'JavaScript', '', '', '', 'suggested');`)
	if err != nil {
		t.Fatal(err)
	}
	update := func(id int64, status string) error {
		return updateProjectService(ctx, tx, 9, id, "", status, "", "")
	}
	for _, status := range []string{"confirmed", "pending", "confirmed"} {
		if err := update(1, status); err != nil {
			t.Fatalf("set generation %s: %v", status, err)
		}
		var generation, profile string
		if err := tx.QueryRow(ctx, `SELECT generation_status,profile_status FROM tgt_service WHERE id=1`).Scan(&generation, &profile); err != nil {
			t.Fatal(err)
		}
		if generation != status || profile != "confirmed" {
			t.Fatalf("generation=%s profile=%s after %s", generation, profile, status)
		}
	}
	if err := update(2, "confirmed"); !errors.Is(err, ErrServiceProfileIncomplete) {
		t.Fatalf("incomplete profile: got %v", err)
	}
	if err := update(99, "confirmed"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing service: got %v", err)
	}
}
