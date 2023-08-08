package wanda

import (
	"errors"
	"testing"

	"bytes"
	"io"
)

func TestWriterToReader(t *testing.T) {
	buf := new(bytes.Buffer)
	buf.WriteString("hello world")

	r := newWriterToReader(buf)
	bs, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(bs) != "hello world" {
		t.Errorf("got %q, want %q", string(bs), "hello world")
	}
}

type errWriterTo struct {
	err error
}

func (w *errWriterTo) WriteTo(wt io.Writer) (int64, error) {
	return 0, w.err
}

func TestWriterToReaderError(t *testing.T) {
	writeErr := errors.New("write error")
	r := newWriterToReader(&errWriterTo{err: writeErr})
	if _, err := io.ReadAll(r); err != writeErr {
		t.Errorf("got %v, want %v", err, writeErr)
	}
}
