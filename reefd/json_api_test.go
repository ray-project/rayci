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

func TestJSONAPIErrorCases(t *testing.T) {
	h := jsonAPI(func(ctx context.Context, req *struct{}) (*struct{}, error) {
		return &struct{}{}, nil
	})

	tests := []struct {
		name             string
		req              *http.Request
		wantCode         int
		wantBodyContains string
	}{
		{
			name:     "method not allowed",
			req:      httptest.NewRequest(http.MethodGet, "/test", nil),
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name: "wrong accept header",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
				req.Header.Set("Accept", "text/html")
				req.Header.Set("Content-Type", reefclient.JSONContentType)
				return req
			}(),
			wantCode:         http.StatusUnsupportedMediaType,
			wantBodyContains: "must accept application/json",
		},
		{
			name: "wrong content type",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
				req.Header.Set("Accept", reefclient.JSONContentType)
				req.Header.Set("Content-Type", "text/plain")
				return req
			}(),
			wantCode:         http.StatusUnsupportedMediaType,
			wantBodyContains: "must use application/json",
		},
		{
			name: "malformed json",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{invalid json`))
				req.Header.Set("Accept", reefclient.JSONContentType)
				req.Header.Set("Content-Type", reefclient.JSONContentType)
				return req
			}(),
			wantCode: http.StatusBadRequest,
		},
		{
			name: "extra data after json",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}extra`))
				req.Header.Set("Accept", reefclient.JSONContentType)
				req.Header.Set("Content-Type", reefclient.JSONContentType)
				return req
			}(),
			wantCode:         http.StatusBadRequest,
			wantBodyContains: "extra data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, tt.req)

			if rec.Code != tt.wantCode {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantCode)
			}
			if tt.wantBodyContains != "" && !strings.Contains(rec.Body.String(), tt.wantBodyContains) {
				t.Errorf("got body %q, want to contain %q", rec.Body.String(), tt.wantBodyContains)
			}
		})
	}
}
