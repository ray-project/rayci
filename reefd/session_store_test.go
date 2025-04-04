package reefd

import (
	"errors"
	"testing"
	"time"

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

func TestSessionStore_insert(t *testing.T) {
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

	now := time.Now().Truncate(time.Second)
	session := &session{
		user:   "testuser",
		token:  "testtoken",
		expire: now.Add(time.Hour * 24),
	}

	if err := s.insert(ctx, session); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := s.get(ctx, session.token)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if got.user != session.user {
		t.Errorf("got user %q, want %q", got.user, session.user)
	}
	if got.expire != session.expire {
		t.Errorf("got expire %q, want %q", got.expire, session.expire)
	}
	if got.token != session.token {
		t.Errorf("got token %q, want %q", got.token, session.token)
	}

	if err := s.delete(ctx, session.token); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, err = s.get(ctx, session.token)
	if err == nil {
		t.Errorf("got nil error after delete, want %v", errSessionNotFound)
	} else if !errors.Is(err, errSessionNotFound) {
		t.Fatalf("got error %v, want %v", err, errSessionNotFound)
	}
}
