package rayapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAnyscaleAPI(t *testing.T) {
	t.Run("missing host", func(t *testing.T) {
		t.Setenv("ANYSCALE_HOST", "")
		t.Setenv("ANYSCALE_CLI_TOKEN", "tok")

		_, err := NewAnyscaleAPI()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ANYSCALE_HOST") {
			t.Errorf("error = %q, want mention of ANYSCALE_HOST", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		t.Setenv("ANYSCALE_HOST", "http://localhost")
		t.Setenv("ANYSCALE_CLI_TOKEN", "")

		_, err := NewAnyscaleAPI()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ANYSCALE_CLI_TOKEN") {
			t.Errorf("error = %q, want mention of ANYSCALE_CLI_TOKEN", err)
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

		t.Setenv("ANYSCALE_HOST", server.URL)
		t.Setenv("ANYSCALE_CLI_TOKEN", "test-token")

		api, err := NewAnyscaleAPI()
		if err != nil {
			t.Fatalf("NewAnyscaleAPI: %v", err)
		}
		if err := api.DeleteWorkspaceByID("expwrk_abc"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}))
		defer server.Close()

		t.Setenv("ANYSCALE_HOST", server.URL)
		t.Setenv("ANYSCALE_CLI_TOKEN", "test-token")

		api, err := NewAnyscaleAPI()
		if err != nil {
			t.Fatalf("NewAnyscaleAPI: %v", err)
		}
		err = api.DeleteWorkspaceByID("expwrk_missing")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("error %q should contain 404", err.Error())
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error %q should contain response body", err.Error())
		}
	})

	t.Run("empty workspace ID", func(t *testing.T) {
		t.Setenv("ANYSCALE_HOST", "http://localhost")
		t.Setenv("ANYSCALE_CLI_TOKEN", "test-token")

		api, err := NewAnyscaleAPI()
		if err != nil {
			t.Fatalf("NewAnyscaleAPI: %v", err)
		}
		err = api.DeleteWorkspaceByID("")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Errorf("error = %q, want mention of empty", err.Error())
		}
	})
}
