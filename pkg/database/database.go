package database

import (
	"context"
	"errors"
	"github.com/0fau/logs/pkg/database/sql"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func doMigrate(dbURL string) error {
	u, err := url.Parse(dbURL)
	if err != nil {
		return err
	}
	u.Scheme = "cockroachdb"

	m, err := migrate.New("file://migrations", u.String())
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

type DB struct {
	pool    *pgxpool.Pool
	queries *sql.Queries
}

func Connect(ctx context.Context, dbURL string) (*DB, error) {
	if err := doMigrate(dbURL); err != nil {
		return nil, err
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &DB{
		pool:    pool,
		queries: sql.New(pool),
	}, nil
}

func pgtext(str string) pgtype.Text {
	return pgtype.Text{String: str, Valid: true}
}
