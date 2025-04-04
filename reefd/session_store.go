package reefd

import (
	"context"
	"database/sql"
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
		CREATE TABLE IF NOT EXISTS sessions (
			token text NOT NULL PRIMARY KEY,
			user text NOT NULL,
			expire integer NOT NULL)`,
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
		ctx, `INSERT INTO sessions (token, user, expire) VALUES (?, ?, ?)`,
		session.token, session.user, session.expire.Unix(),
	)
	return err
}

var errSessionNotFound = errors.New("session not found")

// get a session from the database by token.
func (s *sessionStore) get(ctx context.Context, token string) (*session, error) {
	var user string
	var expire int64

	if err := s.db.Q1(
		ctx, `SELECT user, expire FROM sessions WHERE token = ?`, token,
	).Scan(&user, &expire); err != nil {
		if err == sql.ErrNoRows {
			return nil, errSessionNotFound
		}
		return nil, err
	}

	return &session{user: user, token: token, expire: time.Unix(expire, 0)}, nil
}

// delete a session from the database by token.
func (s *sessionStore) delete(ctx context.Context, token string) error {
	_, err := s.db.X(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// delete expired sessions from the database.
func (s *sessionStore) deleteExpired(ctx context.Context, t time.Time) error {
	_, err := s.db.X(ctx, `DELETE FROM sessions WHERE expire < ?`, t.Unix())
	return err
}
