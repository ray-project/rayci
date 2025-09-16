// Package reefd contains the main logic of the REEf CI service.
package reefd

import (
	"io"
	"net/http"
)

// Config contains the configuration for the running the server.
type Config struct {
}

type server struct {
	config *Config
}

func newServer(c *Config) *server {
	return &server{config: c}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := io.WriteString(w, "Hello, World!")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Serve runs the server.
func Serve(addr string, c *Config) error {
	s := newServer(c)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s,
	}
	return httpServer.ListenAndServe()
}
