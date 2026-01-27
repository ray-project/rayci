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

func TestNewAnyscaleCLI(t *testing.T) {
	cli := NewAnyscaleCLI()
	if cli == nil {
		t.Fatal("expected non-nil AnyscaleCLI")
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
			errContains: "version string too short",
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
			errContains: "version string too short",
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
