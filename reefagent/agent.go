package reefagent

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Agent struct {
	Id    string
	Queue string
	Job   *Job
}

func (a *Agent) Start() {
	// start a loop to ping the server every 1 second on a goroutine until ping returns a job
	for {
		success, job := a.Ping()
		if success {
			a.AcquireAndRunJob(job)
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func (a *Agent) Ping() (bool, *Job) {
	// call POST /ping endpoint with agent ID and queue
	url := fmt.Sprintf("http://localhost:1235/ping?agentId=%s&queue=%s", a.Id, a.Queue)
	resp, err := http.Get(url)

	if err != nil {
		log.Fatal(err)
	}
	var response struct {
		JobId    string   `json:"jobId"`
		Commands []string `json:"commands"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Fatal(err)
	}
	job := &Job{
		Id:       response.JobId,
		Commands: response.Commands,
	}
	defer resp.Body.Close()
	// if response is 200, return the job and true
	// otherwise return nil and false
	if resp.StatusCode == 200 {
		return true, job
	}
	return false, nil
}

func (a *Agent) AcquireAndRunJob(job *Job) {
	success := a.AcquireJob(job.Id)
	if !success {
		fmt.Println("Failed to acquire job")
		return
	}
	a.Job = job
	a.RunJob()
}

func (a *Agent) RunJob() {
	jr := NewJobRunner(a.Job)
	jr.Run()
}

func (a *Agent) AcquireJob(jobId string) bool {
	// Send POST request to acquire the job
	url := fmt.Sprintf("http://localhost:1235/job/acquire?jobId=%s&agentId=%s", jobId, a.Id)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		log.Printf("Error acquiring job: %v", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}
