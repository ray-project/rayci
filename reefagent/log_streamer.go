package reefagent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

type LogChunk struct {
	Data     []byte
	Sequence int
}

type Buffer struct {
	buf []byte
}

func (l *Buffer) Write(b []byte) (int, error) {
	l.buf = append(l.buf, b...)
	return len(b), nil
}

func (l *Buffer) ReadAndFlush() []byte {
	buf := l.buf
	l.buf = []byte{}
	return buf
}

type LogStreamer struct {
	jobId      string
	logsWriter io.Writer
	logs       *Buffer
	logOrder   int
	queue      chan LogChunk
	maxSize    int
	active     bool
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	serviceHost string
}

func NewLogStreamer(jobId string) *LogStreamer {
	ctx, cancel := context.WithCancel(context.Background())
	return &LogStreamer{
		jobId:    jobId,
		logs:     &Buffer{},
		logOrder: 0,
		maxSize:  10 * 1024 * 1024, // 10MB
		active:   false,
		queue:    make(chan LogChunk, 100),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (ls *LogStreamer) Start() {
	ls.active = true
	ls.wg.Add(2)
	go ls.StreamLogs()
	go ls.RetrieveAndUploadChunk()
}

func (ls *LogStreamer) Stop() {
	ls.ChunkLogs(ls.logs.ReadAndFlush())
	ls.cancel()
	ls.wg.Wait()
	close(ls.queue)
}

// StreamLogs streams the result from the logs and chunks them into the queue
func (ls *LogStreamer) StreamLogs() {
	defer ls.wg.Done()
	for {
		// read the logs and chunk them then push into the queue
		logs := ls.logs.ReadAndFlush()
		if len(logs) > 0 {
			ls.ChunkLogs(logs)
		}
		select {
		case <-time.After(1 * time.Second):
		case <-ls.ctx.Done():
			return
		}
	}
}

func (ls *LogStreamer) ChunkLogs(data []byte) {
	chunkSize := ls.maxSize
	for i := 0; i < len(data); i += chunkSize {
		chunkData := data[i:min(i+chunkSize, len(data))]
		logChunk := LogChunk{Data: chunkData, Sequence: ls.logOrder}
		ls.queue <- logChunk
		ls.logOrder++
	}
}

// function that takes chunk from the queue and write it to file
func (ls *LogStreamer) RetrieveAndUploadChunk() {
	defer ls.wg.Done()
	for {
		select {
		case chunk, ok := <-ls.queue:
			if !ok {
				fmt.Println("Queue closed.. exiting")
				return
			}
			// write the chunk to file
			ls.WriteToFile(chunk)
			// send the chunk to server
			ls.UploadChunk(chunk)
		case <-ls.ctx.Done():
			return
		}
	}
}

func (ls *LogStreamer) WriteToFile(logChunk LogChunk) {
	fileName := fmt.Sprintf("logs/%s-%d.log", ls.jobId, logChunk.Sequence)
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("Error writing to file", err)
		return
	}
	defer file.Close()
	file.Write(logChunk.Data)
}

func (ls *LogStreamer) UploadChunk(logChunk LogChunk) {
	// send the chunk to server
	url := fmt.Sprintf("%s/job/logs?jobId=%s&sequence=%d", ls.serviceHost, ls.jobId, logChunk.Sequence)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(logChunk.Data))
	if err != nil {
		fmt.Println("Error creating request", err)
		return
	}
	req.Header.Set("Content-Type", "text/plain")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error uploading chunk", err)
	}
	defer resp.Body.Close()
}
