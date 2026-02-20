package rayapp

import (
	"encoding/json"
	"io"
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

func newTestAPI(t *testing.T, server *httptest.Server) *AnyscaleAPI {
	t.Helper()
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
	return api
}

func TestLaunchTemplateInWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if want := "/api/v2/experimental_workspaces/from_template"; r.URL.Path != want {
				t.Errorf("path = %s, want %s", r.URL.Path, want)
			}
			if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
				t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
			}
			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}
			if payload["template_id"] != "my-template" {
				t.Errorf("template_id = %v, want %q", payload["template_id"], "my-template")
			}
			if payload["cloud_id"] != "cld_123" {
				t.Errorf("cloud_id = %v, want %q", payload["cloud_id"], "cld_123")
			}
			if payload["project_id"] != "prj_456" {
				t.Errorf("project_id = %v, want %q", payload["project_id"], "prj_456")
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"expwrk_789","name":"my-template-20260220"}`))
		}))
		defer server.Close()

		api := newTestAPI(t, server)
		result, err := api.LaunchTemplateInWorkspace("cld_123", "prj_456", "my-template")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["id"] != "expwrk_789" {
			t.Errorf("result[id] = %v, want %q", result["id"], "expwrk_789")
		}
		if result["name"] != "my-template-20260220" {
			t.Errorf("result[name] = %v, want %q", result["name"], "my-template-20260220")
		}
	})

	t.Run("non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid template"}`))
		}))
		defer server.Close()

		api := newTestAPI(t, server)
		_, err := api.LaunchTemplateInWorkspace("cld_123", "prj_456", "bad-template")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("error %q should contain 400", err.Error())
		}
		if !strings.Contains(err.Error(), "invalid template") {
			t.Errorf("error %q should contain response body", err.Error())
		}
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not json`))
		}))
		defer server.Close()

		api := newTestAPI(t, server)
		_, err := api.LaunchTemplateInWorkspace("cld_123", "prj_456", "my-template")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse response") {
			t.Errorf("error %q should contain 'failed to parse response'", err.Error())
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal error"}`))
		}))
		defer server.Close()

		api := newTestAPI(t, server)
		_, err := api.LaunchTemplateInWorkspace("cld_123", "prj_456", "my-template")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("error %q should contain 500", err.Error())
		}
	})
}
