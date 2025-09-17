package reefd

import (
	"context"
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
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
	}()

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
		defer func() {
			if err := rows.Close(); err != nil {
				t.Fatalf("failed to close rows: %v", err)
			}
		}()

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
