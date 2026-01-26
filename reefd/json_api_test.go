package reefd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ray-project/rayci/reefd/reefclient"
)

func TestJSONAPI(t *testing.T) {
	type request struct {
		Message string `json:"message"`
	}

	type response struct {
		Echo string `json:"echo"`
	}

	h := func(ctx context.Context, req *request) (*response, error) {
		if req.Message == "" {
			return nil, errors.New("empty message")
		}
		return &response{Echo: req.Message}, nil
	}

	const callPath = "/api/v1/test"

	mux := http.NewServeMux()
	mux.Handle(callPath, jsonAPI(h))

	s := httptest.NewServer(mux)
	defer s.Close()

	c, err := reefclient.NewJSONCaller(s.URL)
	if err != nil {
		t.Fatalf("failed to create JSON caller: %v", err)
	}

	ctx := context.Background()
	req := &request{Message: "hi"}
	resp := new(response)

	if err := reefclient.JSONCall(
		ctx, c, string(callPath), req, resp,
	); err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	if resp.Echo != req.Message {
		t.Fatalf("got %q, want %q", resp.Echo, req.Message)
	}

	emptyReq := &request{}
	resp = new(response)

	if err := reefclient.JSONCall(
		ctx, c, string(callPath), emptyReq, resp,
	); err == nil {
		t.Fatalf("want error, got nil")
	} else if !strings.Contains(err.Error(), "empty message") {
		t.Fatalf("got unexpected error: %v", err)
	}
}

func TestJSONAPIMethodNotAllowed(t *testing.T) {
	h := jsonAPI(func(ctx context.Context, req *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestJSONAPIWrongAcceptHeader(t *testing.T) {
	h := jsonAPI(func(ctx context.Context, req *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Content-Type", reefclient.JSONContentType)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
	if !strings.Contains(rec.Body.String(), "must accept application/json") {
		t.Errorf("got body %q, want error about Accept header", rec.Body.String())
	}
}

func TestJSONAPIWrongContentType(t *testing.T) {
	h := jsonAPI(func(ctx context.Context, req *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	req.Header.Set("Accept", reefclient.JSONContentType)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
	if !strings.Contains(rec.Body.String(), "must use application/json") {
		t.Errorf("got body %q, want error about Content-Type", rec.Body.String())
	}
}

func TestJSONAPIMalformedJSON(t *testing.T) {
	h := jsonAPI(func(ctx context.Context, req *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{invalid json`))
	req.Header.Set("Accept", reefclient.JSONContentType)
	req.Header.Set("Content-Type", reefclient.JSONContentType)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestJSONAPIExtraData(t *testing.T) {
	h := jsonAPI(func(ctx context.Context, req *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}extra`))
	req.Header.Set("Accept", reefclient.JSONContentType)
	req.Header.Set("Content-Type", reefclient.JSONContentType)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "extra data") {
		t.Errorf("got body %q, want error about extra data", rec.Body.String())
	}
}
