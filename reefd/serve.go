// Package reefd contains the main logic of the REEf CI service.
package reefd

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Config contains the configuration for the running the server.
type Config struct {
}

type server struct {
	reaper *reaper
	config *Config
}

func newServer(ctx context.Context, config *Config) (*server, error) {
	awsClients, err := newAWSClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("new aws clients: %w", err)
	}

	reaper := newReaper(awsClients.ec2())
	return &server{
		reaper: reaper,
		config: config,
	}, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	httpServer := &http.Server{
		Addr:    addr,
		Handler: s,
	}
	go s.background(ctx)

	defer s.Close()
	return httpServer.ListenAndServe()
}
