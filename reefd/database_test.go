package reefd

import (
	"context"
	"errors"
	"testing"
)

type dummyStore struct {
	db *database
}

func (s *dummyStore) create(ctx context.Context) error {
	_, err := s.db.X(ctx, "CREATE TABLE IF NOT EXISTS dummy (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	return err
}

func (s *dummyStore) destroy(ctx context.Context) error {
	_, err := s.db.X(ctx, "DROP TABLE IF EXISTS dummy")
	return err
}

func TestDatabase(t *testing.T) {
	db, err := newSqliteDB("")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	stores := []store{
		&dummyStore{db: db},
	}

	ctx := context.Background()
	if err := createAll(ctx, stores); err != nil {
		t.Fatalf("failed to create all stores: %v", err)
	}

	if _, err = db.X(
		ctx, "INSERT INTO dummy (name) VALUES (?)", "test",
	); err != nil {
		t.Fatalf("failed to insert dummy: %v", err)
	}

	{
		rows, err := db.Q(ctx, "SELECT id, name FROM dummy")
		if err != nil {
			t.Fatalf("failed to query dummy: %v", err)
		}
		defer rows.Close()

		var id int
		var name string
		var names []string
		for rows.Next() {
			if err := rows.Scan(&id, &name); err != nil {
				t.Fatalf("failed to scan dummy: %v", err)
			}
			t.Logf("dummy: %d, %s", id, name)
			names = append(names, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("failed to iterate dummy: %v", err)
		}
		if len(names) != 1 {
			t.Errorf("expected 1 dummy, got %d", len(names))
		}
		if names[0] != "test" {
			t.Errorf("expected dummy name to be test, got %s", names[0])
		}
	}

	{
		row := db.Q1(ctx, "SELECT id, name FROM dummy WHERE id = ?", 1)
		var id int
		var name string
		if err := row.Scan(&id, &name); err != nil {
			t.Fatalf("failed to scan dummy: %v", err)
		}
		t.Logf("dummy: %d, %s", id, name)
		if id != 1 {
			t.Errorf("expected dummy id to be 1, got %d", id)
		}
		if name != "test" {
			t.Errorf("expected dummy name to be test, got %s", name)
		}
	}

	if err := destroyAll(ctx, stores); err != nil {
		t.Fatalf("failed to destroy all stores: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("failed to close database: %v", err)
	}
}

// failingStore is a store that returns configurable errors
type failingStore struct {
	createErr  error
	destroyErr error
}

func (s *failingStore) create(ctx context.Context) error  { return s.createErr }
func (s *failingStore) destroy(ctx context.Context) error { return s.destroyErr }

func TestCreateAllPropagatesError(t *testing.T) {
	wantErr := errors.New("create failed")
	stores := []store{
		&failingStore{createErr: wantErr},
	}

	ctx := context.Background()
	err := createAll(ctx, stores)
	if err != wantErr {
		t.Errorf("createAll() error = %v, want %v", err, wantErr)
	}
}

func TestCreateAllStopsOnFirstError(t *testing.T) {
	firstErr := errors.New("first failed")
	secondErr := errors.New("second failed")
	stores := []store{
		&failingStore{createErr: firstErr},
		&failingStore{createErr: secondErr},
	}

	ctx := context.Background()
	err := createAll(ctx, stores)
	if err != firstErr {
		t.Errorf("createAll() error = %v, want %v (first error)", err, firstErr)
	}
}

func TestDestroyAllPropagatesError(t *testing.T) {
	wantErr := errors.New("destroy failed")
	stores := []store{
		&failingStore{destroyErr: wantErr},
	}

	ctx := context.Background()
	err := destroyAll(ctx, stores)
	if err != wantErr {
		t.Errorf("destroyAll() error = %v, want %v", err, wantErr)
	}
}

func TestDestroyAllStopsOnFirstError(t *testing.T) {
	firstErr := errors.New("first failed")
	secondErr := errors.New("second failed")
	stores := []store{
		&failingStore{destroyErr: firstErr},
		&failingStore{destroyErr: secondErr},
	}

	ctx := context.Background()
	err := destroyAll(ctx, stores)
	if err != firstErr {
		t.Errorf("destroyAll() error = %v, want %v (first error)", err, firstErr)
	}
}

func TestDatabaseDB(t *testing.T) {
	db, err := newSqliteDB("")
	if err != nil {
		t.Fatalf("newSqliteDB() error = %v", err)
	}
	defer db.Close()

	sqlDB := db.DB()
	if sqlDB == nil {
		t.Error("DB() returned nil")
	}

	// Verify the returned DB is functional
	if err := sqlDB.Ping(); err != nil {
		t.Errorf("DB().Ping() error = %v", err)
	}
}

func TestCreateAllEmptyStores(t *testing.T) {
	ctx := context.Background()
	err := createAll(ctx, []store{})
	if err != nil {
		t.Errorf("createAll() with empty stores error = %v, want nil", err)
	}
}

func TestDestroyAllEmptyStores(t *testing.T) {
	ctx := context.Background()
	err := destroyAll(ctx, []store{})
	if err != nil {
		t.Errorf("destroyAll() with empty stores error = %v, want nil", err)
	}
}
