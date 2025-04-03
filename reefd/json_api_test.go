package reefd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

	mux := http.NewServeMux()
	mux.Handle("/api/v1/test", jsonAPI(h))

	s := httptest.NewServer(mux)
	defer s.Close()

	c, err := reefclient.NewJSONCaller(s.URL)
	if err != nil {
		t.Fatalf("failed to create JSON caller: %v", err)
	}

	ctx := context.Background()
	req := &request{Message: "hi"}
	resp := new(response)

	if err := reefclient.JSONCall(ctx, c, "api/v1/test", req, resp); err != nil {
		t.Fatalf("failed to call: %v", err)
	}

	if resp.Echo != req.Message {
		t.Fatalf("got %q, want %q", resp.Echo, req.Message)
	}

	emptyReq := &request{}
	resp = new(response)

	if err := reefclient.JSONCall(ctx, c, "api/v1/test", emptyReq, resp); err == nil {
		t.Fatalf("want error, got nil")
	}
}
