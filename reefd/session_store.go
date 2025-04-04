package reefd

import (
	"context"
	"errors"
	"time"
)

type session struct {
	user   string
	token  string
	expire time.Time
}

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

// destroy the session tables.
func (s *sessionStore) destroy(ctx context.Context) error {
	_, err := s.db.X(ctx, `drop table if exists sessions`)
	return err
}

// insert a session into the database.
func (s *sessionStore) insert(
	ctx context.Context, session *session,
) error {
	_, err := s.db.X(
		ctx, `insert into sessions (token, user, expire) values (?, ?, ?)`,
		session.token, session.user, session.expire.Unix(),
	)
	return err
}

var errSessionNotFound = errors.New("session not found")

// get a session from the database by token.
func (s *sessionStore) get(ctx context.Context, token string) (*session, error) {
	row, err := s.db.Q(
		ctx, `select user, expire from sessions where token = ?`, token,
	)
	if err != nil {
		return nil, err
	}
	defer row.Close()

	if !row.Next() {
		return nil, errSessionNotFound
	}

	var user string
	var expire int64
	if err := row.Scan(&user, &expire); err != nil {
		return nil, err
	}
	return &session{user: user, token: token, expire: time.Unix(expire, 0)}, nil
}

// delete a session from the database by token.
func (s *sessionStore) delete(ctx context.Context, token string) error {
	_, err := s.db.X(ctx, `delete from sessions where token = ?`, token)
	return err
}

// delete expired sessions from the database.
func (s *sessionStore) deleteExpired(ctx context.Context, t time.Time) error {
	_, err := s.db.X(ctx, `delete from sessions where expire < ?`, t.Unix())
	return err
}
