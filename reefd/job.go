package reefd

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Job struct {
	Id        string
	Queue     string
	Commands  []string
	AgentId   string `json:"agent_id,omitempty"`
	CreatedAt time.Time
}

/*
Add jobs table to database:

CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    queue TEXT NOT NULL,
    commands TEXT NOT NULL,
    agent_id TEXT,
    created_at TIMESTAMP NOT NULL
);
*/

func getJob(db *sql.DB, queue string) (*Job, error) {
	job := &Job{}
	var commandsJSON string
	err := db.QueryRow(`SELECT id, queue, commands, created_at FROM jobs WHERE queue = ? AND agent_id IS NULL ORDER BY created_at ASC LIMIT 1`, queue).Scan(&job.Id, &job.Queue, &commandsJSON, &job.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal([]byte(commandsJSON), &job.Commands); err != nil {
		return nil, err
	}
	return job, nil
}
