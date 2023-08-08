package wanda

import (
	"testing"

	"bytes"
	"io"
)

func TestCountingWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	w := newCountingWriter(buf)

	io.WriteString(w, "hello")
	io.WriteString(w, "world")

	if w.n != 10 {
		t.Errorf("expected 10 bytes written, got %d", w.n)
	}
}
