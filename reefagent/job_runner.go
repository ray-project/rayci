package reefagent

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type JobRunner struct {
	logStreamer *LogStreamer
	job         *Job
}

func (jr *JobRunner) Run() {
	jr.logStreamer.Start()
	for _, command := range jr.job.Commands {
		parts := strings.Split(command, " ")
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Stdout = jr.logStreamer.logsWriter
		cmd.Stderr = jr.logStreamer.logsWriter
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error running command %s: %v\n", command, err)
		}
	}
	jr.logStreamer.Stop()
}

func NewJobRunner(job *Job, serviceHost string) *JobRunner {
	jr := &JobRunner{
		job:         job,
		logStreamer: NewLogStreamer(job.Id, serviceHost),
	}
	jr.logStreamer.logsWriter = io.MultiWriter(jr.logStreamer.logs, os.Stdout)
	return jr
}
