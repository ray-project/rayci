package rayapp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// AnyscaleCLI provides methods for interacting with the Anyscale CLI.
type AnyscaleCLI struct{}

const maxOutputBufferSize = 1024 * 1024 // 1 MB

// NewAnyscaleCLI creates a new AnyscaleCLI instance.
func NewAnyscaleCLI() *AnyscaleCLI {
	return &AnyscaleCLI{}
}

func isAnyscaleInstalled() bool {
	_, err := exec.LookPath("anyscale")
	return err == nil
}

// runAnyscaleCLI runs the anyscale CLI with the given arguments.
// Returns the combined output and any error that occurred.
// Output is displayed to the terminal with colors preserved.
func (ac *AnyscaleCLI) runAnyscaleCLI(args []string) (string, error) {
	if !isAnyscaleInstalled() {
		return "", errors.New("anyscale is not installed")
	}

	fmt.Println("anyscale cli args: ", args)
	cmd := exec.Command("anyscale", args...)

	tw := newTailWriter(maxOutputBufferSize)
	cmd.Stdout = io.MultiWriter(os.Stdout, tw)
	cmd.Stderr = io.MultiWriter(os.Stderr, tw)

	err := cmd.Run()
	if err != nil {
		return tw.String(), fmt.Errorf("anyscale error: %w", err)
	}
	return tw.String(), nil
}

// tailWriter is a circular buffer that keeps the most recent `limit`
// bytes. The underlying storage is a fixed-size slice allocated once,
// so memory usage never exceeds the limit.
type tailWriter struct {
	data  []byte
	limit int
	pos   int
	full  bool
}

func newTailWriter(limit int) *tailWriter {
	return &tailWriter{
		data:  make([]byte, limit),
		limit: limit,
	}
}

func (w *tailWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n >= w.limit {
		copy(w.data, p[n-w.limit:])
		w.pos = 0
		w.full = true
		return n, nil
	}
	remaining := w.limit - w.pos
	if n <= remaining {
		copy(w.data[w.pos:], p)
		w.pos += n
		if w.pos == w.limit {
			w.pos = 0
			w.full = true
		}
	} else {
		copy(w.data[w.pos:], p[:remaining])
		copy(w.data, p[remaining:])
		w.pos = n - remaining
		w.full = true
	}
	return n, nil
}

func (w *tailWriter) String() string {
	if !w.full {
		return string(w.data[:w.pos])
	}
	result := make([]byte, w.limit)
	n := copy(result, w.data[w.pos:])
	copy(result[n:], w.data[:w.pos])
	return string(result)
}
