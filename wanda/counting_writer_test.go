package wanda

import (
	"testing"

	"bytes"
	"io"
)

func TestCountingWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	w := newCountingWriter(buf)

	_, err := io.WriteString(w, "hello")
	if err != nil {
		t.Fatalf("write string: %v", err)
	}
	_, err = io.WriteString(w, "world")
	if err != nil {
		t.Fatalf("write string: %v", err)
	}

	if w.n != 10 {
		t.Errorf("expected 10 bytes written, got %d", w.n)
	}
}
