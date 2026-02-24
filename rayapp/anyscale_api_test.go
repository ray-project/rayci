package rayapp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAnyscaleAPI(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		api, err := newAnyscaleAPI("http://localhost", "tok")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if api == nil {
			t.Fatal("got nil, want non-nil anyscaleAPI")
		}
	})

	t.Run("missing host", func(t *testing.T) {
		_, err := newAnyscaleAPI("", "tok")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "host") {
			t.Errorf("error = %q, want mention of host", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := newAnyscaleAPI("http://localhost", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "token") {
			t.Errorf("error = %q, want mention of token", err)
		}
	})
}

func TestDeleteWorkspaceByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want DELETE", r.Method)
			}
			if want := "/api/v2/experimental_workspaces/expwrk_abc"; r.URL.Path != want {
				t.Errorf("path = %s, want %s", r.URL.Path, want)
			}
			if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
				t.Errorf("Authorization = %q, want Bearer test-token", auth)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"deleted"}`))
		}))
		defer server.Close()

		api, err := newAnyscaleAPI(server.URL, "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		if err := api.deleteWorkspaceByID("expwrk_abc"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}))
		defer server.Close()

		api, err := newAnyscaleAPI(server.URL, "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		err = api.deleteWorkspaceByID("expwrk_missing")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var ae *apiError
		if !errors.As(err, &ae) {
			t.Fatalf("error type = %T, want *apiError", err)
		}
		if ae.StatusCode != http.StatusNotFound {
			t.Errorf(
				"StatusCode = %d, want %d",
				ae.StatusCode, http.StatusNotFound,
			)
		}
		if !strings.Contains(ae.Body, "not found") {
			t.Errorf("Body = %q, want mention of not found", ae.Body)
		}
	})

	t.Run("invalid workspace ID", func(t *testing.T) {
		api, err := newAnyscaleAPI("http://localhost", "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		for _, id := range []string{"", "../../admin", "id with spaces", "a/b"} {
			err = api.deleteWorkspaceByID(id)
			if err == nil {
				t.Errorf("deleteWorkspaceByID(%q): expected error, got nil", id)
				continue
			}
			if !strings.Contains(err.Error(), "invalid workspace ID") {
				t.Errorf(
					"deleteWorkspaceByID(%q) error = %q, want invalid workspace ID",
					id, err,
				)
			}
		}
	})
}
