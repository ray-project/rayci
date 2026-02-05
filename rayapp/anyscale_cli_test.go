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
	setupMockAnyscale(t, script)
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

func TestTailWriter(t *testing.T) {
	t.Run("under limit", func(t *testing.T) {
		tw := newTailWriter(16)
		tw.Write([]byte("hello"))
		if got, want := tw.String(), "hello"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("multiple writes under limit", func(t *testing.T) {
		tw := newTailWriter(16)
		tw.Write([]byte("abc"))
		tw.Write([]byte("def"))
		if got, want := tw.String(), "abcdef"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("exact limit", func(t *testing.T) {
		tw := newTailWriter(5)
		tw.Write([]byte("abcde"))
		if got, want := tw.String(), "abcde"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("wraps around keeps tail", func(t *testing.T) {
		tw := newTailWriter(5)
		tw.Write([]byte("abc"))
		tw.Write([]byte("defgh"))
		if got, want := tw.String(), "defgh"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("multiple wraps", func(t *testing.T) {
		tw := newTailWriter(4)
		tw.Write([]byte("ab"))
		tw.Write([]byte("cd"))
		tw.Write([]byte("ef"))
		if got, want := tw.String(), "cdef"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("single write exceeds limit", func(t *testing.T) {
		tw := newTailWriter(4)
		n, err := tw.Write([]byte("abcdefgh"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 8 {
			t.Errorf("Write() = %d, want 8", n)
		}
		if got, want := tw.String(), "efgh"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("write returns full length", func(t *testing.T) {
		tw := newTailWriter(4)
		tw.Write([]byte("abc"))
		n, err := tw.Write([]byte("defgh"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 5 {
			t.Errorf("Write() = %d, want 5", n)
		}
	})

	t.Run("empty write", func(t *testing.T) {
		tw := newTailWriter(4)
		tw.Write([]byte("ab"))
		tw.Write([]byte(""))
		if got, want := tw.String(), "ab"; got != want {
			t.Errorf("String() = %q, want %q", got, want)
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
			wantErr: errors.New("anyscale is not installed"),
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
				if !strings.Contains(err.Error(), tt.wantErr.Error()) {
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
