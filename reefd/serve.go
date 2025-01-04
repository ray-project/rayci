// Package reefd contains the main logic of the REEf CI service.
package reefd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Config contains the configuration for the running the server.
type Config struct {
	DB *sql.DB
}

type server struct {
	config *Config
}

func newServer(c *Config) *server {
	return &server{config: c}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello, World!")
}

// handleLaunchRequest retrieves the desired state from the request body and inserts it into the database
// then starts a goroutine to process the launch requests
func handleLaunchRequest(db *sql.DB, w http.ResponseWriter, r *http.Request, ec2Client EC2Client) {
	var desiredState InstanceInfo
	if err := json.NewDecoder(r.Body).Decode(&desiredState); err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}
	// marshal the desired state to a json string
	desiredJSON, err := json.Marshal(desiredState)
	if err != nil {
		http.Error(w, "Error marshaling desired state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println(string(desiredJSON))
	// insert the desired state into the database
	if _, err := db.Exec(`INSERT INTO launch_requests (desired_state) VALUES (?)`, string(desiredJSON)); err != nil {
		http.Error(w, "Error inserting into database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// start a goroutine to scan the database for launch requests with different desired and current states
	instanceManager := &instanceManager{db: db, ec2Client: ec2Client}
	go processLaunchRequests(instanceManager)
}

// Serve runs the server.
func Serve(addr string, c *Config) error {
	mux := http.NewServeMux()
	ec2Client := getEC2Client()
	mux.HandleFunc("/launch", func(w http.ResponseWriter, r *http.Request) {
		handleLaunchRequest(c.DB, w, r, ec2Client)
	})
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return httpServer.ListenAndServe()
}
