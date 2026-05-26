package rayapp

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWorkspaceStateString(t *testing.T) {
	tests := []struct {
		state WorkspaceState
		want  string
	}{
		{StateTerminated, "TERMINATED"},
		{StateStarting, "STARTING"},
		{StateRunning, "RUNNING"},
		{WorkspaceState(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("WorkspaceState.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetWorkspaceDescription(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeAnyscale{
			workspaces: []*fakeWorkspace{
				{
					ID: "expwrk_abc123", Name: "my-workspace",
					State: "RUNNING",
				},
				{
					ID: "expwrk_xyz789", Name: "my-workspace-2",
					State: "STARTING",
				},
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "get"},
				[]string{"--json"},
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return fake.run(args)
		})

		got, err := cli.getWorkspaceDescription("expwrk_abc123")
		if err != nil {
			t.Fatalf("getWorkspaceDescription() error = %v", err)
		}
		if got["id"] != "expwrk_abc123" {
			t.Errorf("got[%q] = %v, want expwrk_abc123", "id", got["id"])
		}
		if got["name"] != "my-workspace" {
			t.Errorf("got[%q] = %v, want my-workspace", "name", got["name"])
		}
		if got["state"] != "RUNNING" {
			t.Errorf("got[%q] = %v, want RUNNING", "state", got["state"])
		}
	})

	t.Run("CLI failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "get"},
				[]string{"--json"},
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return "", fmt.Errorf("exit status 1")
		})

		_, err := cli.getWorkspaceDescription("expwrk_abc123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get workspace failed") {
			t.Errorf("error %q should contain 'get workspace failed'", err.Error())
		}
	})

	t.Run("invalid JSON output", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "get"},
				[]string{"--json"},
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return "not valid json", nil
		})

		_, err := cli.getWorkspaceDescription("expwrk_abc123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "parse workspace get output") {
			t.Errorf("error %q should contain 'parse workspace get output'", err.Error())
		}
	})
}

func TestCreateEmptyWorkspace(t *testing.T) {
	successRunFunc := (&fakeAnyscale{}).run

	tests := []struct {
		name        string
		runFunc     func(args []string) (string, error)
		config      *WorkspaceTestConfig
		wantPairs   [][2]string
		wantID      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "success without compute config",
			runFunc: successRunFunc,
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantPairs: [][2]string{
				{"--name", "test-workspace"},
				{"--image-uri", "anyscale/ray:2.44.1-py312-cu128"},
				{"--ray-version", "2.44.1"},
			},
			wantID: "expwrk_test-workspace",
		},
		{
			name:    "success with compute config name",
			runFunc: successRunFunc,
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				computeConfig: "basic-single-node-aws",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantPairs: [][2]string{
				{"--name", "test-workspace"},
				{"--image-uri", "anyscale/ray:2.44.1-py312-cu128"},
				{"--ray-version", "2.44.1"},
				{"--compute-config", "basic-single-node-aws"},
			},
			wantID: "expwrk_test-workspace",
		},
		{
			name:    "success with BYOD containerfile",
			runFunc: successRunFunc,
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				buildDir:      filepath.Join("foo", "bar"),
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BYOD: &ClusterEnvBYOD{
							ContainerFile: "Dockerfile",
							RayVersion:    "2.34.0",
						},
					},
				},
			},
			wantPairs: [][2]string{
				{"--name", "test-workspace"},
				{"--containerfile", filepath.Join("foo", "bar", "Dockerfile")},
				{"--ray-version", "2.34.0"},
			},
			wantID: "expwrk_test-workspace",
		},
		{
			name:    "success with BYOD docker image",
			runFunc: successRunFunc,
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BYOD: &ClusterEnvBYOD{
							DockerImage: "my-custom-image:latest",
							RayVersion:  "2.34.0",
						},
					},
				},
			},
			wantPairs: [][2]string{
				{"--name", "test-workspace"},
				{"--image-uri", "my-custom-image:latest"},
				{"--ray-version", "2.34.0"},
			},
			wantID: "expwrk_test-workspace",
		},
		{
			name:    "success with ImageURI",
			runFunc: successRunFunc,
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{ImageURI: "anyscale/ray:2.44.1-py312-cu128"},
				},
			},
			wantPairs: [][2]string{
				{"--name", "test-workspace"},
				{"--image-uri", "anyscale/ray:2.44.1-py312-cu128"},
				{"--ray-version", "2.44.1"},
			},
			wantID: "expwrk_test-workspace",
		},
		{
			name:    "success with ImageURI and compute config",
			runFunc: successRunFunc,
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				computeConfig: "basic-single-node-aws",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						ImageURI: "anyscale/ray:2.35.0-py311",
					},
				},
			},
			wantPairs: [][2]string{
				{"--name", "test-workspace"},
				{"--image-uri", "anyscale/ray:2.35.0-py311"},
				{"--ray-version", "2.35.0"},
				{"--compute-config", "basic-single-node-aws"},
			},
			wantID: "expwrk_test-workspace",
		},
		{
			name:    "nil template",
			runFunc: successRunFunc,
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
			},
			wantErr:     true,
			errContains: "template is required",
		},
		{
			name:    "invalid build ID",
			runFunc: successRunFunc,
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
			name: "CLI error",
			runFunc: func(args []string) (string, error) {
				return "", fmt.Errorf("exit status 1")
			},
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantPairs: [][2]string{
				{"--name", "test-workspace"},
				{"--image-uri", "anyscale/ray:2.44.1-py312-cu128"},
				{"--ray-version", "2.44.1"},
			},
			wantErr:     true,
			errContains: "create empty workspace failed",
		},
		{
			name: "create output missing id",
			runFunc: func(args []string) (string, error) {
				return "Workspace created successfully", nil
			},
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantPairs: [][2]string{
				{"--name", "test-workspace"},
				{"--image-uri", "anyscale/ray:2.44.1-py312-cu128"},
				{"--ray-version", "2.44.1"},
			},
			wantErr:     true,
			errContains: "could not parse workspace id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := NewAnyscaleCLI()
			cli.setRunFunc(func(args []string) (string, error) {
				checkArgs(t, args, []string{"workspace_v2", "create"}, nil, tt.wantPairs)
				return tt.runFunc(args)
			})

			gotID, err := cli.createEmptyWorkspace(tt.config)

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
			if gotID != tt.wantID {
				t.Errorf("createEmptyWorkspace() id = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

func TestPushFolderToWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "push"},
				nil,
				[][2]string{
					{"--id", "expwrk_abc123"},
					{"--local-dir", "/local/path"},
				},
			)
			return fake.run(args)
		})

		err := cli.pushFolderToWorkspace("expwrk_abc123", "/local/path")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "push"},
				nil,
				[][2]string{
					{"--id", "expwrk_abc123"},
					{"--local-dir", "/local/path"},
				},
			)
			return "", fmt.Errorf("exit status 1")
		})

		err := cli.pushFolderToWorkspace("expwrk_abc123", "/local/path")
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
		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "terminate"},
				nil,
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return fake.run(args)
		})

		err := cli.terminateWorkspace("expwrk_abc123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "terminate"},
				nil,
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return "", fmt.Errorf("exit status 1")
		})

		err := cli.terminateWorkspace("expwrk_abc123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(
			err.Error(), "terminate workspace failed",
		) {
			t.Errorf(
				"error %q should contain"+
					" 'terminate workspace failed'",
				err.Error(),
			)
		}
	})
}

func TestRunCmdInWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeAnyscale{}
		pairs := [][2]string{{"--id", "expwrk_abc123"}}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args, []string{"workspace_v2", "run_command"}, nil, pairs)
			positional := findPositionalArgs(args[2:], nil, pairs)
			want := []string{"echo hello"}
			if !slices.Equal(positional, want) {
				t.Errorf("positional args = %v, want %v", positional, want)
			}
			return fake.run(args)
		})

		err := cli.runCmdInWorkspace("expwrk_abc123", "echo hello")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		pairs := [][2]string{{"--id", "expwrk_abc123"}}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args, []string{"workspace_v2", "run_command"}, nil, pairs)
			positional := findPositionalArgs(args[2:], nil, pairs)
			want := []string{"failing-command"}
			if !slices.Equal(positional, want) {
				t.Errorf("positional args = %v, want %v", positional, want)
			}
			return "", fmt.Errorf("exit status 1")
		})

		err := cli.runCmdInWorkspace("expwrk_abc123", "failing-command")
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
		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(
				t,
				args,
				[]string{"workspace_v2", "start"},
				nil,
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return fake.run(args)
		})

		err := cli.startWorkspace("expwrk_abc123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "start"},
				nil,
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return "", fmt.Errorf("exit status 1")
		})

		err := cli.startWorkspace("expwrk_abc123")
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
		fake := &fakeAnyscale{
			workspaces: []*fakeWorkspace{{
				ID: "expwrk_abc123", Name: "test-workspace", State: "RUNNING",
			}},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(
				t,
				args,
				[]string{"workspace_v2", "status"},
				nil,
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return fake.run(args)
		})

		output, err := cli.getWorkspaceStatus("expwrk_abc123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "RUNNING") {
			t.Errorf("output %q should contain 'RUNNING'", output)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "status"},
				nil,
				[][2]string{{"--id", "expwrk_abc123"}},
			)
			return "", fmt.Errorf("exit status 1")
		})

		_, err := cli.getWorkspaceStatus("expwrk_abc123")
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
		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "wait"},
				nil,
				[][2]string{
					{"--id", "expwrk_abc123"},
					{"--state", "RUNNING"},
				},
			)
			return fake.run(args)
		})

		output, err := cli.waitForWorkspaceState(
			"expwrk_abc123", StateRunning,
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "reached target state") {
			t.Errorf(
				"output %q should contain"+
					" 'reached target state'",
				output,
			)
		}
		if !strings.Contains(output, "RUNNING") {
			t.Errorf(
				"output %q should contain 'RUNNING'",
				output,
			)
		}
	})

	t.Run("wait for terminated", func(t *testing.T) {
		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "wait"},
				nil,
				[][2]string{
					{"--id", "expwrk_abc123"},
					{"--state", "TERMINATED"},
				},
			)
			return fake.run(args)
		})

		output, err := cli.waitForWorkspaceState(
			"expwrk_abc123", StateTerminated,
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "reached target state") {
			t.Errorf(
				"output %q should contain"+
					" 'reached target state'",
				output,
			)
		}
		if !strings.Contains(output, "TERMINATED") {
			t.Errorf(
				"output %q should contain 'TERMINATED'",
				output,
			)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"workspace_v2", "wait"},
				nil,
				[][2]string{
					{"--id", "expwrk_abc123"},
					{"--state", "RUNNING"},
				},
			)
			return "", fmt.Errorf("exit status 1")
		})

		_, err := cli.waitForWorkspaceState(
			"expwrk_abc123", StateRunning,
		)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(
			err.Error(),
			"wait for workspace state failed",
		) {
			t.Errorf(
				"error %q should contain"+
					" 'wait for workspace state failed'",
				err.Error(),
			)
		}
	})
}
