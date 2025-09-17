package reefclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONCaller(t *testing.T) {
	type message struct {
		Message string `json:"message"`
	}

	h := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/test" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Accept") != JSONContentType {
			http.Error(w, "wrong accept", http.StatusUnsupportedMediaType)
			return
		}
		w.Header().Set("Content-Type", JSONContentType)
		_, err := io.WriteString(w, `{"message":"hello"}`)
		if err != nil {
			t.Fatalf("write string: %v", err)
		}
	}

	s := httptest.NewServer(http.HandlerFunc(h))
	defer s.Close()

	c, err := NewJSONCaller(s.URL)
	if err != nil {
		t.Fatalf("failed to create JSON caller: %v", err)
	}

	ctx := context.Background()
	req := &message{Message: "hi"}
	resp := new(message)

	if err := JSONCall(ctx, c, "api/v1/test", req, resp); err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	if resp.Message != "hello" {
		t.Fatalf("got %q, want `hello`", resp.Message)
	}
}
