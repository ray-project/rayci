package rayapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestScript_Generate(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*TestScript)
		contains []string
	}{
		{
			name:  "default script",
			setup: func(s *TestScript) {},
			contains: []string{
				"#!/bin/bash",
				"set -e",
				"pip install nbmake",
				"pytest --nbmake .",
			},
		},
		{
			name: "with extra pip package",
			setup: func(s *TestScript) {
				s.WithPipPackage("pytest-xdist")
			},
			contains: []string{
				"pip install nbmake pytest-xdist",
			},
		},
		{
			name: "with extra pytest arg",
			setup: func(s *TestScript) {
				s.WithPytestArg("-v")
			},
			contains: []string{
				"pytest --nbmake -v .",
			},
		},
		{
			name: "with custom target dir",
			setup: func(s *TestScript) {
				s.WithTargetDir("notebooks/")
			},
			contains: []string{
				"pytest --nbmake notebooks/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewTestScript()
			tt.setup(s)
			output := s.Generate()

			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q, got:\n%s", want, output)
				}
			}
		})
	}
}

func TestTestScript_Save(t *testing.T) {
	t.Run("creates executable file", func(t *testing.T) {
		tmp := t.TempDir()
		s := NewTestScript()

		err := s.Save(tmp)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		path := filepath.Join(tmp, "test-notebooks.sh")

		// Check file exists
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("file not created: %v", err)
		}

		// Check executable permission
		if info.Mode().Perm()&0100 == 0 {
			t.Error("file should be executable")
		}

		// Check content
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		if !strings.Contains(string(content), "#!/bin/bash") {
			t.Error("file should contain shebang")
		}
		if !strings.Contains(string(content), "pip install nbmake") {
			t.Error("file should contain pip install")
		}
		if !strings.Contains(string(content), "pytest --nbmake .") {
			t.Error("file should contain pytest command")
		}
	})

	t.Run("fails on invalid directory", func(t *testing.T) {
		s := NewTestScript()
		err := s.Save("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("expected error for invalid directory")
		}
	})
}
