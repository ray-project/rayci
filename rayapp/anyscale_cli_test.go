package rayapp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeAnyscale writes a fake anyscale script to a temp directory
// and returns its path. If script is empty, returns a path that does not exist.
func writeFakeAnyscale(t *testing.T, script string) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "anyscale")

	if script == "" {
		return bin // non-existent path
	}

	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake script: %v", err)
	}
	return bin
}

func TestNewAnyscaleCLI(t *testing.T) {
	cli := NewAnyscaleCLI()
	if cli == nil {
		t.Fatal("expected non-nil AnyscaleCLI")
	}
}

func TestIsAnyscaleInstalled(t *testing.T) {
	t.Run("not installed", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "")}
		if cli.isAnyscaleInstalled() {
			t.Error("should return false when binary does not exist")
		}
	})

	t.Run("installed", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho mock")}
		if !cli.isAnyscaleInstalled() {
			t.Error("should return true when binary exists")
		}
	})
}

func TestRunAnyscaleCLI_BufferTruncation(t *testing.T) {
	// Generate output larger than maxOutputBufferSize (1 MB).
	// Use a pattern that lets us verify we kept the tail, not the head.
	// Each line is 100 bytes (including newline), we need >10485 lines for >1MB.
	script := `#!/bin/bash
for ((i=1; i<=12000; i++)); do
    printf "LINE_%05d_" "$i"
    printf "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX\n"
done
`
	cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

	output, err := cli.runAnyscaleCLI([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output) > maxOutputBufferSize {
		t.Errorf("output length = %d, want <= %d", len(output), maxOutputBufferSize)
	}

	// Verify we kept the tail: last line should be LINE_12000
	if !strings.Contains(output, "LINE_12000_") {
		t.Error("output should contain the last line (LINE_12000_)")
	}

	// Verify the head was truncated: first line should NOT be present
	if strings.Contains(output, "LINE_00001_") {
		t.Error("output should not contain the first line (LINE_00001_)")
	}
}

func TestWorkspaceStateString(t *testing.T) {
	tests := []struct {
		state WorkspaceState
		want  string
	}{
		{StateTerminated, "TERMINATED"},
		{StateStarting, "STARTING"},
		{StateRunning, "RUNNING"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("WorkspaceState.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunAnyscaleCLI(t *testing.T) {
	tests := []struct {
		name       string
		script     string
		args       []string
		wantErrStr string
		wantSubstr string
	}{
		{
			name:       "anyscale not installed",
			script:     "", // non-existent binary
			args:       []string{"--version"},
			wantErrStr: "anyscale is not installed",
		},
		{
			name:       "success",
			script:     "#!/bin/sh\necho \"output: $@\"",
			args:       []string{"service", "deploy"},
			wantSubstr: "output: service deploy",
		},
		{
			name:       "empty args",
			script:     "#!/bin/sh\necho \"help\"",
			args:       []string{},
			wantSubstr: "help",
		},
		{
			name:       "command fails with stderr",
			script:     "#!/bin/sh\necho \"error msg\" >&2; exit 1",
			args:       []string{"deploy"},
			wantSubstr: "error msg",
			wantErrStr: "anyscale error",
		},
		{
			name:       "exec failed with exit code in output",
			script:     "#!/bin/sh\necho \"exec failed with exit code 1\"; exit 0",
			args:       []string{"deploy"},
			wantErrStr: "anyscale error: command failed:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, tt.script)}

			output, err := cli.runAnyscaleCLI(tt.args)

			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrStr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.wantErrStr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantSubstr != "" && !strings.Contains(output, tt.wantSubstr) {
				t.Errorf("output %q should contain %q", output, tt.wantSubstr)
			}
		})
	}
}

func TestCreateEmptyWorkspace(t *testing.T) {
	tests := []struct {
		name          string
		script        string
		config        *WorkspaceTestConfig
		wantErr       bool
		errContains   string
		wantArgSubstr string
	}{
		{
			name:   "success without compute config",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantArgSubstr: "workspace_v2 create",
		},
		{
			name:   "success with compute config name",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				computeConfig: "basic-single-node-aws",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantArgSubstr: "--compute-config",
		},
		{
			name:   "success with BYOD containerfile",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				buildFile:     filepath.Join("foo", "bar", "BUILD.yaml"),
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BYOD: &ClusterEnvBYOD{
							ContainerFile: "Dockerfile",
							RayVersion:    "2.34.0",
						},
					},
				},
			},
			wantArgSubstr: "--containerfile",
		},
		{
			name:   "success with ImageURI",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						ImageURI: "anyscale/ray:2.44.1-py312-cu128",
					},
				},
			},
			wantArgSubstr: "--image-uri anyscale/ray:2.44.1-py312-cu128",
		},
		{
			name:   "success with ImageURI and compute config",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				computeConfig: "basic-single-node-aws",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						ImageURI: "anyscale/ray:2.35.0-py311",
					},
				},
			},
			wantArgSubstr: "--image-uri anyscale/ray:2.35.0-py311",
		},
		{
			name:   "invalid build ID",
			script: "#!/bin/sh\necho \"args: $@\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "invalid-build-id",
					},
				},
			},
			wantErr:     true,
			errContains: "cluster env",
		},
		{
			name:   "invalid ImageURI",
			script: "#!/bin/sh\necho \"args: $@\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						ImageURI: "other/ray:2.44.1-py312",
					},
				},
			},
			wantErr:     true,
			errContains: "cluster env",
		},
		{
			name:   "CLI error",
			script: "#!/bin/sh\nexit 1",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantErr:     true,
			errContains: "create empty workspace failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupMockAnyscale(t, tt.script)
			cli := NewAnyscaleCLI()

			workspaceID, err := cli.createEmptyWorkspace(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if workspaceID == "" {
				t.Error("expected workspace ID, got empty string")
			}
		})
	}
}

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

		cli := NewAnyscaleCLI()
		err := cli.deleteWorkspaceByID("expwrk_123")
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

		cli := NewAnyscaleCLI()
		err := cli.deleteWorkspaceByID("expwrk_123")
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

		cli := NewAnyscaleCLI()
		err := cli.deleteWorkspaceByID("expwrk_abc")
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

		cli := NewAnyscaleCLI()
		err := cli.deleteWorkspaceByID("expwrk_missing")
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

func TestPushFolderToWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"push $@\"")
		cli := NewAnyscaleCLI()

		err := cli.pushFolderToWorkspace("my-workspace", "/local/path")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()

		err := cli.pushFolderToWorkspace("my-workspace", "/local/path")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "push file to workspace failed") {
			t.Errorf("error %q should contain 'push file to workspace failed'", err.Error())
		}
	})
}

func TestTerminateWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"terminating $@\"")
		cli := NewAnyscaleCLI()

		err := cli.terminateWorkspace("my-workspace")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()

		err := cli.terminateWorkspace("my-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "terminate workspace failed") {
			t.Errorf("error %q should contain 'terminate workspace failed'", err.Error())
		}
	})
}

func TestRunCmdInWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"running: $@\"")
		cli := NewAnyscaleCLI()

		err := cli.runCmdInWorkspace("test-workspace", "echo hello")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()

		err := cli.runCmdInWorkspace("test-workspace", "failing-command")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "run command in workspace failed") {
			t.Errorf("error %q should contain 'run command in workspace failed'", err.Error())
		}
	})
}

func TestStartWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"starting $@\"")
		cli := NewAnyscaleCLI()

		err := cli.startWorkspace("test-workspace")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()

		err := cli.startWorkspace("test-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "start workspace failed") {
			t.Errorf("error %q should contain 'start workspace failed'", err.Error())
		}
	})
}

func TestGetWorkspaceStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"RUNNING\"")
		cli := NewAnyscaleCLI()

		output, err := cli.getWorkspaceStatus("test-workspace")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "RUNNING") {
			t.Errorf("output %q should contain 'RUNNING'", output)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()

		_, err := cli.getWorkspaceStatus("test-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get workspace state failed") {
			t.Errorf("error %q should contain 'get workspace state failed'", err.Error())
		}
	})
}

func TestWaitForWorkspaceState(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"state reached\"")
		cli := NewAnyscaleCLI()

		output, err := cli.waitForWorkspaceState("test-workspace", StateRunning)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "state reached") {
			t.Errorf("output %q should contain 'state reached'", output)
		}
	})

	t.Run("wait for terminated", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"terminated\"")
		cli := NewAnyscaleCLI()

		output, err := cli.waitForWorkspaceState("test-workspace", StateTerminated)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "terminated") {
			t.Errorf("output %q should contain 'terminated'", output)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()

		_, err := cli.waitForWorkspaceState("test-workspace", StateRunning)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "wait for workspace state failed") {
			t.Errorf("error %q should contain 'wait for workspace state failed'", err.Error())
		}
	})
}
