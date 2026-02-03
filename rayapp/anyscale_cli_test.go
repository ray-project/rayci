package rayapp

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// setupMockAnyscale creates a mock anyscale script and returns a cleanup function.
func setupMockAnyscale(t *testing.T, script string) {
	t.Helper()
	tmp := t.TempDir()

	if script != "" {
		mockScript := tmp + "/anyscale"
		if err := os.WriteFile(mockScript, []byte(script), 0755); err != nil {
			t.Fatalf("failed to create mock script: %v", err)
		}
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", tmp)
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

func TestConvertBuildIdToImageURI(t *testing.T) {
	tests := []struct {
		name           string
		buildId        string
		wantImageURI   string
		wantRayVersion string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid build ID with suffix",
			buildId:        "anyscaleray2441-py312-cu128",
			wantImageURI:   "anyscale/ray:2.44.1-py312-cu128",
			wantRayVersion: "2.44.1",
		},
		{
			name:           "valid build ID without suffix",
			buildId:        "anyscaleray2440",
			wantImageURI:   "anyscale/ray:2.44.0",
			wantRayVersion: "2.44.0",
		},
		{
			name:           "valid build ID with only python suffix",
			buildId:        "anyscaleray2350-py311",
			wantImageURI:   "anyscale/ray:2.35.0-py311",
			wantRayVersion: "2.35.0",
		},
		{
			name:           "valid build ID version 3",
			buildId:        "anyscaleray3001-py312",
			wantImageURI:   "anyscale/ray:3.00.1-py312",
			wantRayVersion: "3.00.1",
		},
		{
			name:        "invalid prefix",
			buildId:     "rayimage2441-py312",
			wantErr:     true,
			errContains: "must start with",
		},
		{
			name:        "version too short",
			buildId:     "anyscaleray123",
			wantErr:     true,
			errContains: "major(1 digit)",
		},
		{
			name:        "empty build ID",
			buildId:     "",
			wantErr:     true,
			errContains: "must start with",
		},
		{
			name:        "only prefix",
			buildId:     "anyscaleray",
			wantErr:     true,
			errContains: "major(1 digit)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imageURI, rayVersion, err := convertBuildIdToImageURI(tt.buildId)

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
			if imageURI != tt.wantImageURI {
				t.Errorf("imageURI = %q, want %q", imageURI, tt.wantImageURI)
			}
			if rayVersion != tt.wantRayVersion {
				t.Errorf("rayVersion = %q, want %q", rayVersion, tt.wantRayVersion)
			}
		})
	}
}

func TestConvertImageURIToBuildID(t *testing.T) {
	tests := []struct {
		name           string
		imageURI       string
		wantBuildID    string
		wantRayVersion string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid image URI with suffix",
			imageURI:       "anyscale/ray:2.44.1-py312-cu128",
			wantBuildID:    "anyscaleray2441-py312-cu128",
			wantRayVersion: "2.44.1",
		},
		{
			name:           "valid image URI without suffix",
			imageURI:       "anyscale/ray:2.44.0",
			wantBuildID:    "anyscaleray2440",
			wantRayVersion: "2.44.0",
		},
		{
			name:           "valid image URI with python suffix",
			imageURI:       "anyscale/ray:2.35.0-py311",
			wantBuildID:    "anyscaleray2350-py311",
			wantRayVersion: "2.35.0",
		},
		{
			name:        "invalid prefix",
			imageURI:    "other/ray:2.44.1-py312",
			wantErr:     true,
			errContains: "must start with",
		},
		{
			name:        "invalid version format",
			imageURI:    "anyscale/ray:2.4.1",
			wantErr:     true,
			errContains: "major(1 digit)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildID, rayVersion, err := convertImageURIToBuildID(tt.imageURI)
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
			if buildID != tt.wantBuildID {
				t.Errorf("buildID = %q, want %q", buildID, tt.wantBuildID)
			}
			if rayVersion != tt.wantRayVersion {
				t.Errorf("rayVersion = %q, want %q", rayVersion, tt.wantRayVersion)
			}
		})
	}
}

func TestGetImageURIAndRayVersionFromClusterEnv(t *testing.T) {
	tests := []struct {
		name           string
		env            *ClusterEnv
		wantImageURI   string
		wantRayVersion string
		wantErr        bool
		errContains    string
	}{
		{
			name: "only BuildID",
			env:  &ClusterEnv{BuildID: "anyscaleray2441-py312-cu128"},
			wantImageURI: "anyscale/ray:2.44.1-py312-cu128", wantRayVersion: "2.44.1",
		},
		{
			name: "only ImageURI",
			env:  &ClusterEnv{ImageURI: "anyscale/ray:2.44.1-py312-cu128"},
			wantImageURI: "anyscale/ray:2.44.1-py312-cu128", wantRayVersion: "2.44.1",
		},
		{
			name:         "both set",
			env:          &ClusterEnv{BuildID: "anyscaleray2440", ImageURI: "anyscale/ray:2.44.0"},
			wantErr:      true,
			errContains:  "exactly one",
		},
		{
			name:         "neither set",
			env:          &ClusterEnv{},
			wantErr:      true,
			errContains:  "exactly one",
		},
		{
			name:         "nil env",
			env:          nil,
			wantErr:      true,
			errContains:  "cluster_env is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imageURI, rayVersion, err := getImageURIAndRayVersionFromClusterEnv(tt.env)
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
			if imageURI != tt.wantImageURI {
				t.Errorf("imageURI = %q, want %q", imageURI, tt.wantImageURI)
			}
			if rayVersion != tt.wantRayVersion {
				t.Errorf("rayVersion = %q, want %q", rayVersion, tt.wantRayVersion)
			}
		})
	}
}

func TestRunAnyscaleCLI(t *testing.T) {
	tests := []struct {
		name       string
		script     string
		args       []string
		wantErr    error
		wantSubstr string
	}{
		{
			name:    "anyscale not installed",
			script:  "", // empty PATH, no script
			args:    []string{"--version"},
			wantErr: errAnyscaleNotInstalled,
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
			wantErr:    errors.New("anyscale error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupMockAnyscale(t, tt.script)
			cli := NewAnyscaleCLI()

			output, err := cli.runAnyscaleCLI(tt.args)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if errors.Is(tt.wantErr, errAnyscaleNotInstalled) {
					if !errors.Is(err, errAnyscaleNotInstalled) {
						t.Errorf("expected errAnyscaleNotInstalled, got: %v", err)
					}
				} else if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("error %q should contain %q", err.Error(), tt.wantErr.Error())
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

func TestIsAnyscaleInstalled(t *testing.T) {
	t.Run("not installed", func(t *testing.T) {
		setupMockAnyscale(t, "")
		if isAnyscaleInstalled() {
			t.Error("should return false when not in PATH")
		}
	})

	t.Run("installed", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho mock")
		if !isAnyscaleInstalled() {
			t.Error("should return true when in PATH")
		}
	})
}

func TestParseComputeConfigName(t *testing.T) {
	tests := []struct {
		name           string
		awsConfigPath  string
		wantConfigName string
	}{
		{
			name:           "basic-single-node config",
			awsConfigPath:  "configs/basic-single-node/aws.yaml",
			wantConfigName: "basic-single-node-aws",
		},
		{
			name:           "simple configs directory",
			awsConfigPath:  "configs/aws.yaml",
			wantConfigName: "configs-aws",
		},
		{
			name:           "nested directory",
			awsConfigPath:  "configs/compute/production/aws.yaml",
			wantConfigName: "production-aws",
		},
		{
			name:           "gcp config",
			awsConfigPath:  "configs/basic-single-node/gcp.yaml",
			wantConfigName: "basic-single-node-gcp",
		},
		{
			name:           "yaml extension",
			awsConfigPath:  "configs/my-config/aws.yaml",
			wantConfigName: "my-config-aws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseComputeConfigName(tt.awsConfigPath)
			if got != tt.wantConfigName {
				t.Errorf("parseComputeConfigName(%q) = %q, want %q", tt.awsConfigPath, got, tt.wantConfigName)
			}
		})
	}
}

func TestCreateComputeConfig(t *testing.T) {
	t.Run("creates when config does not exist", func(t *testing.T) {
		// Mock: get fails (not found), create succeeds
		script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "get" ]; then
    echo "config not found"
    exit 1
fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    echo "created compute config: $@"
    exit 0
fi
exit 1
`
		setupMockAnyscale(t, script)
		cli := NewAnyscaleCLI()

		// Create a temporary config file with new format (no conversion needed)
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString("head_node:\n  instance_type: m5.xlarge\n")
		tmpFile.Close()

		output, err := cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "compute-config create") {
			t.Errorf("output %q should contain 'compute-config create'", output)
		}
		if !strings.Contains(output, "-n my-config") {
			t.Errorf("output %q should contain '-n my-config'", output)
		}
	})

	t.Run("skips creation when config exists", func(t *testing.T) {
		// Mock: get succeeds (config found)
		script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "get" ]; then
    echo "name: my-config"
    exit 0
fi
exit 1
`
		setupMockAnyscale(t, script)
		cli := NewAnyscaleCLI()

		output, err := cli.CreateComputeConfig("my-config", "/path/to/config.yaml")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "name: my-config") {
			t.Errorf("output %q should contain 'name: my-config'", output)
		}
		// Should NOT contain create since it was skipped
		if strings.Contains(output, "compute-config create") {
			t.Errorf("output %q should NOT contain 'compute-config create' when config exists", output)
		}
	})

	t.Run("failure when create fails", func(t *testing.T) {
		// Mock: get fails (not found), create also fails
		script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "get" ]; then
    exit 1
fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    exit 1
fi
exit 1
`
		setupMockAnyscale(t, script)
		cli := NewAnyscaleCLI()

		// Create a temporary config file with new format
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString("head_node:\n  instance_type: m5.xlarge\n")
		tmpFile.Close()

		_, err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "create compute config failed") {
			t.Errorf("error %q should contain 'create compute config failed'", err.Error())
		}
	})
}

func TestGetComputeConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"name: my-config\nhead_node:\n  instance_type: m5.xlarge\"")
		cli := NewAnyscaleCLI()

		output, err := cli.GetComputeConfig("my-config")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "name: my-config") {
			t.Errorf("output %q should contain 'name: my-config'", output)
		}
	})

	t.Run("success with version", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"args: $@\"")
		cli := NewAnyscaleCLI()

		output, err := cli.GetComputeConfig("my-config:2")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "-n my-config:2") {
			t.Errorf("output %q should contain '-n my-config:2'", output)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()

		_, err := cli.GetComputeConfig("nonexistent-config")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get compute config failed") {
			t.Errorf("error %q should contain 'get compute config failed'", err.Error())
		}
	})
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
		if !strings.Contains(err.Error(), "delete workspace failed") {
			t.Errorf("error %q should contain 'delete workspace failed'", err.Error())
		}
	})
}

func TestCopyTemplateToWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"pushing $@\"")
		cli := NewAnyscaleCLI()
		config := &WorkspaceTestConfig{
			workspaceName: "test-workspace",
			template: &Template{
				Dir: "/path/to/template",
			},
		}

		err := cli.copyTemplateToWorkspace(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()
		config := &WorkspaceTestConfig{
			workspaceName: "test-workspace",
			template: &Template{
				Dir: "/path/to/template",
			},
		}

		err := cli.copyTemplateToWorkspace(config)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "copy template to workspace failed") {
			t.Errorf("error %q should contain 'copy template to workspace failed'", err.Error())
		}
	})
}

func TestRunCmdInWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\necho \"running: $@\"")
		cli := NewAnyscaleCLI()
		config := &WorkspaceTestConfig{
			workspaceName: "test-workspace",
		}

		err := cli.runCmdInWorkspace(config, "echo hello")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()
		config := &WorkspaceTestConfig{
			workspaceName: "test-workspace",
		}

		err := cli.runCmdInWorkspace(config, "failing-command")
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
		config := &WorkspaceTestConfig{
			workspaceName: "test-workspace",
		}

		err := cli.startWorkspace(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		setupMockAnyscale(t, "#!/bin/sh\nexit 1")
		cli := NewAnyscaleCLI()
		config := &WorkspaceTestConfig{
			workspaceName: "test-workspace",
		}

		err := cli.startWorkspace(config)
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
