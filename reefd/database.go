package reefd

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"
)

type store interface {
	create(ctx context.Context) error
	destroy(ctx context.Context) error
}

func createAll(ctx context.Context, stores []store) error {
	for _, s := range stores {
		if err := s.create(ctx); err != nil {
			return err
		}
	}
	return nil
}

func destroyAll(ctx context.Context, stores []store) error {
	for _, s := range stores {
		if err := s.destroy(ctx); err != nil {
			return err
		}
	}
	return nil
}

type database struct {
	driver string
	db     *sql.DB
}

func newSqliteDB(f string) (*database, error) {
	if f == "" {
		f = ":memory:"
	}

	const driver = "sqlite"
	db, err := sql.Open(driver, f)
	if err != nil {
		return nil, err
	}

	return &database{db: db}, nil
}

func (d *database) DB() *sql.DB { return d.db }

func (d *database) Close() error {
	return d.db.Close()
}

// X is a shortcut for ExecContext.
func (d *database) X(ctx context.Context, q string, args ...any) (
	sql.Result, error,
) {
	return d.db.ExecContext(ctx, q, args...)
}

// Q is a shortcut for QueryContext.
func (d *database) Q(ctx context.Context, q string, args ...any) (
	*sql.Rows, error,
) {
	return d.db.QueryContext(ctx, q, args...)
}

// Q1 is a shortcut for QueryRowContext.
func (d *database) Q1(ctx context.Context, q string, args ...any) *sql.Row {
	return d.db.QueryRowContext(ctx, q, args...)
}
