package reefclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		io.WriteString(w, `{"message":"hello"}`)
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

func TestNewJSONCallerInvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"control character in URL", "http://example.com\x00"},
		{"invalid scheme", "://missing-scheme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewJSONCaller(tt.url)
			if err == nil {
				t.Error("NewJSONCaller() error = nil, want error for invalid URL")
			}
		})
	}
}

func TestJSONCallerHTTPErrors(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		wantErrContain string
	}{
		{
			name: "server returns 500 with error message",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "internal server error", http.StatusInternalServerError)
			},
			wantErrContain: "500",
		},
		{
			name: "server returns 404 with empty body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErrContain: "404",
		},
		{
			name: "server returns wrong content type",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("not json"))
			},
			wantErrContain: "content type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(tt.handler)
			defer s.Close()

			c, err := NewJSONCaller(s.URL)
			if err != nil {
				t.Fatalf("NewJSONCaller() error = %v", err)
			}

			ctx := context.Background()
			_, err = c.call(ctx, "/test", []byte(`{}`))
			if err == nil {
				t.Fatal("call() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErrContain) {
				t.Errorf("call() error = %q, want error containing %q", err, tt.wantErrContain)
			}
		})
	}
}

func TestJSONCallDecodeError(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", JSONContentType)
		w.Write([]byte(`{"incomplete": `)) // malformed JSON
	}

	s := httptest.NewServer(http.HandlerFunc(h))
	defer s.Close()

	c, err := NewJSONCaller(s.URL)
	if err != nil {
		t.Fatalf("NewJSONCaller() error = %v", err)
	}

	type response struct {
		Message string `json:"message"`
	}

	ctx := context.Background()
	resp := new(response)
	err = JSONCall(ctx, c, "/test", &struct{}{}, resp)
	if err == nil {
		t.Fatal("JSONCall() error = nil, want decode error")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("JSONCall() error = %q, want error containing 'decode response'", err)
	}
}

func TestJSONCallUnknownField(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", JSONContentType)
		w.Write([]byte(`{"known": "value", "unknown": "extra"}`))
	}

	s := httptest.NewServer(http.HandlerFunc(h))
	defer s.Close()

	c, err := NewJSONCaller(s.URL)
	if err != nil {
		t.Fatalf("NewJSONCaller() error = %v", err)
	}

	type response struct {
		Known string `json:"known"`
	}

	ctx := context.Background()
	resp := new(response)
	err = JSONCall(ctx, c, "/test", &struct{}{}, resp)
	if err == nil {
		t.Fatal("JSONCall() error = nil, want error for unknown field")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("JSONCall() error = %q, want error containing 'decode response'", err)
	}
}
