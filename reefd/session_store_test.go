package reefd

import (
	"testing"

	"context"
	"path/filepath"
)

func TestSessionStore_lifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	db, err := newSqliteDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}

	s := newSessionStore(db)

	if err := s.create(ctx); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.destroy(ctx); err != nil {
		t.Fatalf("destroy: %v", err)
	}
}
