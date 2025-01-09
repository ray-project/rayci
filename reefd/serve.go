// Package reefd contains the main logic of the REEf CI service.
package reefd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
	// insert the desired state into the database
	if _, err := db.Exec(`INSERT INTO launch_requests (desired_state) VALUES (?)`, string(desiredJSON)); err != nil {
		http.Error(w, "Error inserting into database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// start a goroutine to scan the database for launch requests with different desired and current states
	instanceManager := &instanceManager{db: db, ec2Client: ec2Client}
	go processLaunchRequests(instanceManager)
}

// handleJobLogs handles job logs sent from the agent
func handleJobLogs(w http.ResponseWriter, r *http.Request) {
	jobId := r.URL.Query().Get("jobId")
	sequence := r.URL.Query().Get("sequence")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading body: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println("log ", jobId, "-", sequence, ": ", string(body))
	// TODO: figure out how to store and display logs in order and a nice way
}

// handlePing handles requests from the agent to check if there's any job for agent to take
func handlePing(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// get agent ID from the request
	queue := r.URL.Query().Get("queue")
	// Look into database to see if there's any job that is in the queue
	// If there's any job, send it to the agent
	job := getJob(db, queue)
	// send the jobId and job commands back in response
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":    job.Id,
		"commands": job.Commands,
	})
}

// handleAcquireJob handles requests from the agent to acquire a job
func handleAcquireJob(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// get agentId and jobId from the request
	agentId := r.URL.Query().Get("agentId")
	jobId := r.URL.Query().Get("jobId")
	// update the job with the agent ID
	if _, err := db.Exec(`UPDATE jobs SET agent_id = ? WHERE id = ?`, agentId, jobId); err != nil {
		http.Error(w, "Error updating job: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// send a 200 response
	fmt.Println("Agent", agentId, "acquired job", jobId)
	w.WriteHeader(http.StatusOK)
}

// handleJobAdd handles requests to add a job
func handleJobAdd(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading body: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// decompose body into job
	var job Job
	if err := json.Unmarshal(body, &job); err != nil {
		http.Error(w, "Error unmarshalling job: "+err.Error(), http.StatusInternalServerError)
		return
	}
	job.CreatedAt = time.Now()
	// insert the job into the database
	commandsJson, err := json.Marshal(job.Commands)
	if err != nil {
		http.Error(w, "Error marshalling commands: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := db.Exec(`INSERT INTO jobs (commands, queue, created_at) VALUES (?, ?, ?)`, string(commandsJson), job.Queue, job.CreatedAt); err != nil {
		http.Error(w, "Error inserting into database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// send a 200 response
	w.WriteHeader(http.StatusOK)
}

// Serve runs the server.
func Serve(addr string, c *Config) error {
	mux := http.NewServeMux()
	ec2Client := getEC2Client()
	mux.HandleFunc("/instances/launch", func(w http.ResponseWriter, r *http.Request) {
		handleLaunchRequest(c.DB, w, r, ec2Client)
	})
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		handlePing(c.DB, w, r)
	})
	mux.HandleFunc("/job/logs", func(w http.ResponseWriter, r *http.Request) {
		handleJobLogs(w, r)
	})
	mux.HandleFunc("/job/acquire", func(w http.ResponseWriter, r *http.Request) {
		handleAcquireJob(c.DB, w, r)
	})
	mux.HandleFunc("/job/add", func(w http.ResponseWriter, r *http.Request) {
		handleJobAdd(c.DB, w, r)
	})
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return httpServer.ListenAndServe()
}
