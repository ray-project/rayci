// Package reefd contains the main logic of the REEf CI service.
package reefd

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

// Config contains the configuration for the running the server.
type Config struct {
}

type server struct {
	reaper *reaper
	config *Config
}

const awsRegion = "us-west-2"

func newServer(ctx context.Context, config *Config) (*server, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(
		ctx, awsconfig.WithRegion(awsRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	reaper := newReaper(awsConfig)
	return &server{
		reaper: reaper,
		config: config,
	}, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Ray CI")
}

func (s *server) background(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Minute)

	for t := range ticker.C {
		log.Print("Tick: ", t)

		if err := s.reaper.listAndReapDeadWindowsInstances(ctx); err != nil {
			log.Println("terminateDeadWindowsMachines: ", err)
		}
	}
}

func (s *server) Close() error {
	return nil
}

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
