package raycilint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdFilelength_ConfigAndOverride(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "config limit exceeded",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "override raises limit",
			args:    []string{"-config-value", "max_lines=100"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			configContent := strings.Join([]string{
				"go_filelength:",
				"  max_lines: 10",
			}, "\n") + "\n"
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(
				configPath, []byte(configContent), 0644,
			); err != nil {
				t.Fatalf("write config: %v", err)
			}

			cfg, err := loadConfig(configPath)
			if err != nil {
				t.Fatalf("loadConfig() error: %v", err)
			}

			goFile := filepath.Join(tmpDir, "big.go")
			if err := os.WriteFile(
				goFile,
				[]byte(strings.Repeat("package x\n", 20)),
				0644,
			); err != nil {
				t.Fatalf("write go file: %v", err)
			}

			oldWd, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(oldWd)

			err = cmdFilelength(cfg, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"cmdFilelength() error = %v, wantErr %v",
					err, tt.wantErr,
				)
			}
		})
	}
}

func TestOverrideKeysHelp(t *testing.T) {
	got := overrideKeysHelp(coverageConfig{})
	want := "      min_coverage_pct (float64)"
	if got != want {
		t.Errorf(
			"overrideKeysHelp() = %q, want %q", got, want,
		)
	}
}

func TestApplyOverrides_Filelength(t *testing.T) {
	cfg := newConfig()
	err := applyOverrides(cfg.Filelength, []string{"max_lines=42"})
	if err != nil {
		t.Fatalf("applyOverrides() error: %v", err)
	}
	if got, want := cfg.Filelength.MaxLines, 42; got != want {
		t.Errorf("MaxLines = %d, want %d", got, want)
	}
}

func TestApplyOverrides_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown key", "min_coverage_pct=90"},
		{"bad format", "noequalssign"},
		{"bad value", "max_lines=notanint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newConfig()
			err := applyOverrides(
				cfg.Filelength, []string{tt.input},
			)
			if err == nil {
				t.Fatalf(
					"applyOverrides(%q) = nil, want error",
					tt.input,
				)
			}
		})
	}
}

func TestApplyOverrides_Coverage(t *testing.T) {
	cfg := newConfig()
	err := applyOverrides(
		cfg.Coverage, []string{"min_coverage_pct=95.5"},
	)
	if err != nil {
		t.Fatalf("applyOverrides() error: %v", err)
	}
	if got, want := cfg.Coverage.MinCoveragePct, 95.5; got != want {
		t.Errorf("MinCoveragePct = %f, want %f", got, want)
	}
}
