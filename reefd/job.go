package reefd

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Command struct {
	Cmd  string
	Args []string
}

type Job struct {
	Id        string
	Queue     string
	Commands  []string
	AgentId   string `json:"agent_id,omitempty"`
	CreatedAt time.Time
}

func getJob(db *sql.DB, queue string) *Job {
	job := &Job{}
	var commandsJSON string
	err := db.QueryRow(`SELECT id, queue, commands, created_at FROM jobs WHERE queue = ? AND agent_id IS NULL ORDER BY created_at ASC LIMIT 1`, queue).Scan(&job.Id, &job.Queue, &commandsJSON, &job.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return nil // Also return nil on other DB errors
	}
	// unmarshal the commandsJSON into job.Commands
	json.Unmarshal([]byte(commandsJSON), &job.Commands)
	return job
}
