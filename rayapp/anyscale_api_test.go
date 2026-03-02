package rayapp

import (
	"encoding/json"
	"errors"
	"io"
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
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("method = %s, want DELETE", r.Method)
				}
				wantPath := "/api/v2/experimental_workspaces/expwrk_abc"
				if r.URL.Path != wantPath {
					t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
				}
				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-token" {
					t.Errorf("Authorization = %q, want Bearer test-token", auth)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"deleted"}`))
			},
		))
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
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":"not found"}`))
			},
		))
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
			t.Errorf("StatusCode = %d, want %d", ae.StatusCode, http.StatusNotFound)
		}
		if !strings.Contains(ae.Body, "not found") {
			t.Errorf("Body = %q, want mention of not found", ae.Body)
		}
	})

	t.Run("no Content-Type header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ct := r.Header.Get("Content-Type")
				if ct != "" {
					t.Errorf("Content-Type = %q, want empty for DELETE", ct)
				}
				w.WriteHeader(http.StatusOK)
			},
		))
		defer server.Close()

		api, err := newAnyscaleAPI(server.URL, "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		if err := api.deleteWorkspaceByID("expwrk_abc"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid workspace ID", func(t *testing.T) {
		api, err := newAnyscaleAPI("http://localhost", "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		ids := []string{"", "../../admin", "id with spaces", "a/b"}
		for _, id := range ids {
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

func TestLaunchTemplateInWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				wantPath := "/api/v2/experimental_workspaces/from_template"
				if r.URL.Path != wantPath {
					t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
				}
				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-token" {
					t.Errorf("Authorization = %q, want Bearer test-token", auth)
				}
				ct := r.Header.Get("Content-Type")
				if ct != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", ct)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got := payload["template_id"]; got != "my-tmpl" {
					t.Errorf("template_id = %v, want my-tmpl", got)
				}
				if got := payload["cloud_id"]; got != "cld_1" {
					t.Errorf("cloud_id = %v, want cld_1", got)
				}
				if got := payload["project_id"]; got != "prj_1" {
					t.Errorf("project_id = %v, want prj_1", got)
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"result": map[string]any{
						"id":   "ws_123",
						"name": "my-tmpl-20260101",
					},
				})
			},
		))
		defer server.Close()

		api, err := newAnyscaleAPI(server.URL, "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		result, err := api.launchTemplateInWorkspace("cld_1", "prj_1", "my-tmpl", "my-ws")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := result["id"]; got != "ws_123" {
			t.Errorf("result[id] = %v, want ws_123", got)
		}
		if got := result["name"]; got != "my-tmpl-20260101" {
			t.Errorf("result[name] = %v, want my-tmpl-20260101", got)
		}
	})

	t.Run("non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"bad request"}`))
			},
		))
		defer server.Close()

		api, err := newAnyscaleAPI(server.URL, "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		_, err = api.launchTemplateInWorkspace("cld_1", "prj_1", "my-tmpl", "my-ws")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("error = %q, want mention of 400", err)
		}
	})

	t.Run("missing result key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"data": "something"})
			},
		))
		defer server.Close()

		api, err := newAnyscaleAPI(server.URL, "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		_, err = api.launchTemplateInWorkspace("cld_1", "prj_1", "my-tmpl", "my-ws")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "missing 'result' key") {
			t.Errorf("error = %q, want mention of missing 'result' key", err)
		}
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`not json`))
			},
		))
		defer server.Close()

		api, err := newAnyscaleAPI(server.URL, "test-token")
		if err != nil {
			t.Fatalf("newAnyscaleAPI: %v", err)
		}
		_, err = api.launchTemplateInWorkspace("cld_1", "prj_1", "my-tmpl", "my-ws")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse response body") {
			t.Errorf("error = %q, want mention of failed to parse response body", err)
		}
	})
}
