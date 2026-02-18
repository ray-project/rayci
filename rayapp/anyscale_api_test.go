package rayapp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDeleteWorkspaceByID(t *testing.T) {
	t.Run("ANYSCALE_HOST not set", func(t *testing.T) {
		origHost := os.Getenv("ANYSCALE_HOST")
		origToken := os.Getenv("ANYSCALE_CLI_TOKEN")
		t.Cleanup(func() {
			os.Setenv("ANYSCALE_HOST", origHost)
			os.Setenv("ANYSCALE_CLI_TOKEN", origToken)
		})
		os.Unsetenv("ANYSCALE_HOST")
		os.Setenv("ANYSCALE_CLI_TOKEN", "token")

		_, err := newAnyscaleAPI()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ANYSCALE_HOST") {
			t.Errorf("error %q should contain ANYSCALE_HOST", err.Error())
		}
	})

	t.Run("ANYSCALE_CLI_TOKEN not set", func(t *testing.T) {
		origHost := os.Getenv("ANYSCALE_HOST")
		origToken := os.Getenv("ANYSCALE_CLI_TOKEN")
		t.Cleanup(func() {
			os.Setenv("ANYSCALE_HOST", origHost)
			os.Setenv("ANYSCALE_CLI_TOKEN", origToken)
		})
		os.Setenv("ANYSCALE_HOST", "https://api.example.com")
		os.Unsetenv("ANYSCALE_CLI_TOKEN")

		_, err := newAnyscaleAPI()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ANYSCALE_CLI_TOKEN") {
			t.Errorf("error %q should contain ANYSCALE_CLI_TOKEN", err.Error())
		}
	})

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

		origHost := os.Getenv("ANYSCALE_HOST")
		origToken := os.Getenv("ANYSCALE_CLI_TOKEN")
		t.Cleanup(func() {
			os.Setenv("ANYSCALE_HOST", origHost)
			os.Setenv("ANYSCALE_CLI_TOKEN", origToken)
		})
		os.Setenv("ANYSCALE_HOST", server.URL)
		os.Setenv("ANYSCALE_CLI_TOKEN", "test-token")

		api, err := newAnyscaleAPI()
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		err = api.DeleteWorkspaceByID("expwrk_abc")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}))
		defer server.Close()

		origHost := os.Getenv("ANYSCALE_HOST")
		origToken := os.Getenv("ANYSCALE_CLI_TOKEN")
		t.Cleanup(func() {
			os.Setenv("ANYSCALE_HOST", origHost)
			os.Setenv("ANYSCALE_CLI_TOKEN", origToken)
		})
		os.Setenv("ANYSCALE_HOST", server.URL)
		os.Setenv("ANYSCALE_CLI_TOKEN", "test-token")

		api, err := newAnyscaleAPI()
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
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
}
