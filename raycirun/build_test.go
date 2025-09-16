package raycirun

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"
)

func TestBuild(t *testing.T) {
	const org = "ray-project"
	const pipeline = "microcheck"
	pipelinePath := path.Join(
		"/v2",
		"organizations", org,
		"pipelines", pipeline,
	)

	const buildURL = "https://bk.com/ray-project/microcheck/builds/123456"
	apiServer := func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path

		switch {
		case r.Method == "POST" && p == path.Join(pipelinePath, "builds"):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			log.Printf("body: %s", string(body))

			var buildReq struct {
				Commit  string `json:"commit"`
				Branch  string `json:"branch"`
				Message string `json:"message"`

				Env map[string]string `json:"env"`

				IgnoreBranchFilters bool `json:"ignore_pipeline_branch_filters"`
			}

			if err := json.Unmarshal(body, &buildReq); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if !buildReq.IgnoreBranchFilters {
				http.Error(
					w,
					"ignore_pipeline_branch_filters not set",
					http.StatusBadRequest,
				)
				return
			}

			buildResp := struct {
				ID     string `json:"id"`
				WebURL string `json:"web_url"`
			}{
				ID:     "123456",
				WebURL: buildURL,
			}
			jsonResp, err := json.Marshal(buildResp)
			if err != nil {
				t.Fatalf("marshal build response: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(jsonResp)
			if err != nil {
				t.Fatalf("write build response: %v", err)
			}

		default:
			log.Printf("unexpected request: %s %s", r.Method, r.URL.String())
			http.Error(w, "not found", http.StatusNotFound)
		}
	}

	s := httptest.NewServer(http.HandlerFunc(apiServer))
	defer s.Close()

	serverURL, err := url.Parse(s.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	b := &Build{
		ServerBaseURL: serverURL,
		Org:           "ray-project",
		Pipeline:      "microcheck",

		PR:      "42",
		Tags:    []string{"foo", "bar"},
		Selects: []string{"baz"},
	}

	ctx := context.Background()
	build, err := b.Create(ctx, "test-token")
	if err != nil {
		t.Fatalf("create build: %v", err)
	}

	if build.ID != "123456" {
		t.Fatalf("got build id %q, want 123456", build.ID)
	}

	if build.WebURL != buildURL {
		t.Fatalf("got build URL %q, want %q", build.WebURL, buildURL)
	}
}

func TestBuildOrgAlias(t *testing.T) {
	for _, test := range []struct {
		org  string
		want string
	}{
		{"r", "ray-project"},
		{"ray", "ray-project"},
		{"as", "anyscale"},
		{"p", "anyscale"},
	} {
		b := &Build{Org: test.org}
		if got := b.orgName(); got != test.want {
			t.Errorf("orgName(%q) = %q, want %q", test.org, got, test.want)
		}
	}
}

func TestBranchName(t *testing.T) {
	for _, test := range []struct {
		pr     string
		branch string
		want   string
	}{
		{"", "main", "main"},
		{"123", "", "refs/pull/123/head"},
		{"123", "main", "refs/pull/123/head"},
	} {
		b := &Build{PR: test.pr, Branch: test.branch}
		if got := b.branchName(); got != test.want {
			t.Errorf(
				"branchName(%q, %q) = %q, want %q",
				test.pr, test.branch, got, test.want,
			)
		}
	}
}

func TestSelectStr(t *testing.T) {
	for _, test := range []struct {
		tags    []string
		selects []string
		want    string
	}{{
		tags:    []string{},
		selects: []string{},
		want:    "",
	}, {
		tags:    []string{"foo", "bar"},
		selects: []string{},
		want:    "tag:foo,tag:bar",
	}, {
		tags:    []string{},
		selects: []string{"baz"},
		want:    "baz",
	}} {
		b := &Build{Tags: test.tags, Selects: test.selects}
		if got := b.selectStr(); got != test.want {
			t.Errorf(
				"selectStr(%v, %v) = %q, want %q",
				test.tags, test.selects,
				got, test.want,
			)
		}
	}
}
