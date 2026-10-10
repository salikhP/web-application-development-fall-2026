package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("POSTGRES_DB"),
	)

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := initSchema(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}

	return db, nil
}

func initSchema(ctx context.Context, db *pgxpool.Pool) error {
	query := `
		CREATE TABLE IF NOT EXISTS docker_events (
			id BIGSERIAL PRIMARY KEY,
			container_id TEXT NOT NULL,
			container_name TEXT,
			image TEXT,
			action TEXT NOT NULL,
			compose_project TEXT,
			compose_service TEXT,
			occurred_at TIMESTAMPTZ NOT NULL
		);
	`

	_, err := db.Exec(ctx, query)
	return err
}
