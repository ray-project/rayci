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
	// Next testing this
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
