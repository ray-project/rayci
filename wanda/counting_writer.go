package wanda

import (
	"io"
)

type countingWriter struct {
	w io.Writer
	n int64
}

func newCountingWriter(w io.Writer) *countingWriter {
	return &countingWriter{w: w}
}

// Write writes to the underlying writer and counts the number of bytes written.
func (w *countingWriter) Write(buf []byte) (int, error) {
	n, err := w.w.Write(buf)
	w.n += int64(n)
	return n, err
}
