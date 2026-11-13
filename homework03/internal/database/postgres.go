package database

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	databaseURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD")),
		Host:     net.JoinHostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")),
		Path:     "/" + os.Getenv("POSTGRES_DB"),
		RawQuery: "sslmode=disable",
	}

	db, err := pgxpool.New(ctx, databaseURL.String())
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if _, err := db.Exec(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}

	return db, nil
}
