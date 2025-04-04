package reefd

import (
	"context"
)

type sessionStore struct {
	db *database
}

func newSessionStore(db *database) *sessionStore {
	return &sessionStore{db: db}
}

// create the session tables if it doesn't exist.
func (s *sessionStore) create(ctx context.Context) error {
	_, err := s.db.X(ctx, `
		create table if not exists sessions (
			token text not null primary key,
			user text not null,
			expire integer not null)`,
	)
	return err
}

func (s *sessionStore) destroy(ctx context.Context) error {
	_, err := s.db.X(ctx, `drop table if exists sessions`)
	return err
}
