package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	appmigrations "github.com/openware-io/open-green-pass/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	dsn := os.Getenv("GP_DB_DSN")
	if dsn == "" {
		log.Error("migration configuration", "error", "GP_DB_DSN is required")
		os.Exit(2)
	}

	if err := run(dsn); err != nil {
		log.Error("apply database migrations", "error", err)
		os.Exit(1)
	}
	log.Info("database migrations are current")
}

func run(dsn string) error {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse GP_DB_DSN: %w", err)
	}
	database := stdlib.OpenDB(*config)
	defer database.Close()
	if _, err := database.Exec(`CREATE SCHEMA IF NOT EXISTS gp`); err != nil {
		return fmt.Errorf("create GreenPass schema: %w", err)
	}

	sourceDriver, err := iofs.New(appmigrations.Files, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	// GreenPass owns the gp schema. The shared PostgreSQL instance also hosts
	// Temporal, whose public.schema_migrations is unrelated; pinning migrate's
	// metadata table to gp prevents one service's migration state from being
	// mistaken for the other's.
	databaseDriver, err := postgres.WithInstance(database, &postgres.Config{SchemaName: "gp"})
	if err != nil {
		return fmt.Errorf("open postgres migration driver: %w", err)
	}
	runner, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", databaseDriver)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer runner.Close()

	if err := runner.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
