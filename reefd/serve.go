// Package reefd contains the main logic of the REEf CI service.
package reefd

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Config contains the configuration for the running the server.
type Config struct {
	Database string

	DisableBackground bool

	UserKeys map[string]string
}

type server struct {
	db *database

	stores []store

	reaper *reaper
	config *Config

	apiV1 http.Handler
}

func newServer(ctx context.Context, config *Config) (*server, error) {
	db, err := newSqliteDB(config.Database)
	if err != nil {
		return nil, fmt.Errorf("new sqlite db: %w", err)
	}

	awsClients, err := newAWSClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("new aws clients: %w", err)
	}
	reaper := newReaper(awsClients.ec2())

	sessionStore := newSessionStore(db)
	authGate := newAuthGate(
		sessionStore,
		config.UserKeys,
		[]string{
			"/api/v1/login",
			"/api/v1/logout",
		}, // unauthenticated endpoints
	)

	apiMux := http.NewServeMux()
	apiMux.Handle("/api/v1/login", jsonAPI(authGate.apiLogin))
	apiMux.Handle("/api/v1/logout", jsonAPI(authGate.apiLogout))

	stores := []store{sessionStore}

	return &server{
		db:     db,
		stores: stores,
		reaper: reaper,
		config: config,
		apiV1:  authGate.gate(apiMux),
	}, nil
}

func (s *server) initStorage(ctx context.Context) error {
	return createAll(ctx, s.stores)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		s.apiV1.ServeHTTP(w, r)
		return
	}

	io.WriteString(w, "Ray CI")
}

func (s *server) listAndReapDeadWindowsInstances(ctx context.Context) error {
	for {
		n, err := s.reaper.listAndReapDeadWindowsInstances(ctx)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}

		time.Sleep(5 * time.Second)
	}
}

func (s *server) background(ctx context.Context) {
	log.Println("background process started")

	if err := s.listAndReapDeadWindowsInstances(ctx); err != nil {
		log.Println("listAndReapDeadWindowsInstances: ", err)
	}

	const period = 20 * time.Minute
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.listAndReapDeadWindowsInstances(ctx); err != nil {
			log.Println("listAndReapDeadWindowsInstances: ", err)
		}
	}
}

func (s *server) Close() error { return nil }

// Serve runs the server.
func Serve(ctx context.Context, addr string, config *Config) error {
	s, err := newServer(ctx, config)
	if err != nil {
		return fmt.Errorf("new server: %w", err)
	}
	if err := s.initStorage(ctx); err != nil {
		return fmt.Errorf("init storage: %w", err)
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: s,
	}

	if !config.DisableBackground {
		go s.background(ctx)
	}

	defer s.Close()
	return httpServer.ListenAndServe()
}
