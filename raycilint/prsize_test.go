package raycilint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := strings.Join([]string{
		"prsize:",
		"  max_additions: 500",
		"  max_deletions: 1000",
		"  ignore:",
		"    - vendor/",
	}, "\n") + "\n"

	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	policy, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if policy.Prsize == nil {
		t.Fatal("loadConfig().Size = nil")
	}
	if got, want := policy.Prsize.MaxAdditions, 500; got != want {
		t.Errorf("MaxAdditions = %d, want %d", got, want)
	}
	if got, want := policy.Prsize.MaxDeletions, 1000; got != want {
		t.Errorf("MaxDeletions = %d, want %d", got, want)
	}
	if got, want := len(policy.Prsize.Ignore), 1; got != want {
		t.Errorf("len(Ignore) = %d, want %d", got, want)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := loadConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("loadConfig() error = nil, want error for missing file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte(":\n\t:bad"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := loadConfig(path)
	if err == nil {
		t.Fatal("loadConfig() error = nil, want error for invalid YAML")
	}
}

func TestParseDiffNumstat(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		ignore     []string
		wantAdded  int
		wantDelete int
	}{
		{
			name: "MultiFile",
			output: strings.Join([]string{
				"10\t5\tsrc/main.go",
				"3\t0\tsrc/util.go",
			}, "\n") + "\n",
			wantAdded:  13,
			wantDelete: 5,
		},
		{
			name: "IgnoredPrefix",
			output: strings.Join([]string{
				"100\t50\tvendor/lib.go",
				"3\t1\tsrc/main.go",
			}, "\n") + "\n",
			ignore:     []string{"vendor/"},
			wantAdded:  3,
			wantDelete: 1,
		},
		{
			name: "BinaryFile",
			output: strings.Join([]string{
				"-\t-\timage.png",
				"5\t2\tsrc/main.go",
			}, "\n") + "\n",
			wantAdded:  5,
			wantDelete: 2,
		},
		{
			name:       "Empty",
			output:     "",
			wantAdded:  0,
			wantDelete: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats, err := parseDiffNumstat([]byte(tt.output), tt.ignore)
			if err != nil {
				t.Fatalf("parseDiffNumstat() error: %v", err)
			}
			if got, want := stats.linesAdded, tt.wantAdded; got != want {
				t.Errorf("added = %d, want %d", got, want)
			}
			if got, want := stats.linesDeleted, tt.wantDelete; got != want {
				t.Errorf("deleted = %d, want %d", got, want)
			}
		})
	}
}

func TestCheckSize(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *prsizeConfig
		stats        *diffStats
		wantCount    int
		wantContains string
	}{
		{
			name:      "UnderThreshold",
			cfg:       &prsizeConfig{MaxAdditions: 100, MaxDeletions: 100},
			stats:     &diffStats{linesAdded: 50, linesDeleted: 50},
			wantCount: 0,
		},
		{
			name:         "ExceedsAdditions",
			cfg:          &prsizeConfig{MaxAdditions: 100, MaxDeletions: 1000},
			stats:        &diffStats{linesAdded: 200, linesDeleted: 50},
			wantCount:    1,
			wantContains: "additions",
		},
		{
			name:      "ExceedsBoth",
			cfg:       &prsizeConfig{MaxAdditions: 100, MaxDeletions: 100},
			stats:     &diffStats{linesAdded: 200, linesDeleted: 200},
			wantCount: 2,
		},
		{
			name:      "ZeroThreshold",
			cfg:       &prsizeConfig{MaxAdditions: 0, MaxDeletions: 0},
			stats:     &diffStats{linesAdded: 9999, linesDeleted: 9999},
			wantCount: 0,
		},
		{
			name:         "OnlyAdditions",
			cfg:          &prsizeConfig{MaxAdditions: 100},
			stats:        &diffStats{linesAdded: 200, linesDeleted: 9999},
			wantCount:    1,
			wantContains: "additions",
		},
		{
			name:         "OnlyDeletions",
			cfg:          &prsizeConfig{MaxDeletions: 100},
			stats:        &diffStats{linesAdded: 9999, linesDeleted: 200},
			wantCount:    1,
			wantContains: "deletions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkSize(tt.cfg, tt.stats)
			if len(got) != tt.wantCount {
				t.Fatalf("checkSize() returned %d warnings, want %d", len(got), tt.wantCount)
			}
			if tt.wantContains != "" && !strings.Contains(got[0], tt.wantContains) {
				t.Errorf("checkSize()[0] = %q, want %s warning", got[0], tt.wantContains)
			}
		})
	}
}
