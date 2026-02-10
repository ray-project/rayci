package rayapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupFakeAnyscale creates a fake anyscale script and returns a cleanup function.
func setupFakeAnyscale(t *testing.T, script string) {
	t.Helper()
	tmp := t.TempDir()

	if script != "" {
		mockScript := filepath.Join(tmp, "anyscale")
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
		setupFakeAnyscale(t, "")
		if isAnyscaleInstalled() {
			t.Error("should return false when not in PATH")
		}
	})

	t.Run("installed", func(t *testing.T) {
		setupFakeAnyscale(t, "#!/bin/sh\necho mock")
		if !isAnyscaleInstalled() {
			t.Error("should return true when in PATH")
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
	setupFakeAnyscale(t, script)
	cli := NewAnyscaleCLI()

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

func TestRunAnyscaleCLI(t *testing.T) {
	tests := []struct {
		name       string
		script     string
		args       []string
		wantErrStr string
		wantSubstr string
	}{
		{
			name:    "anyscale not installed",
			script:  "", // empty PATH, no script
			args:    []string{"--version"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupFakeAnyscale(t, tt.script)
			cli := NewAnyscaleCLI()

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
