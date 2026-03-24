package rayapp

import (
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
			wantErrStr: "error msg",
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

func TestRunAnyscaleCLI_StderrNotInOutput(t *testing.T) {
	script := strings.Join([]string{
		"#!/bin/sh",
		`echo "[WARNING] upgrade your CLI" >&2`,
		`echo '{"id": "ws-123"}'`,
	}, "\n")
	cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

	output, err := cli.runAnyscaleCLI([]string{"workspace_v2", "get"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output, "WARNING") {
		t.Errorf("stdout output should not contain stderr warning, got %q", output)
	}
	if !strings.Contains(output, `{"id": "ws-123"}`) {
		t.Errorf("stdout output should contain JSON payload, got %q", output)
	}
}
