package rayapp

import (
	"os"
	"path/filepath"
	"strings"
)

// TestScript generates a shell script to run notebook tests.
type TestScript struct {
	pipPackages []string
	pytestArgs  []string
	targetDir   string
}

// NewTestScript creates a new TestScript with default nbmake configuration.
func NewTestScript() *TestScript {
	return &TestScript{
		pipPackages: []string{"nbmake"},
		pytestArgs:  []string{"--nbmake"},
		targetDir:   ".",
	}
}

// WithPipPackage adds an additional pip package to install.
func (s *TestScript) WithPipPackage(pkg string) *TestScript {
	s.pipPackages = append(s.pipPackages, pkg)
	return s
}

// WithPytestArg adds an additional pytest argument.
func (s *TestScript) WithPytestArg(arg string) *TestScript {
	s.pytestArgs = append(s.pytestArgs, arg)
	return s
}

// WithTargetDir sets the target directory for pytest.
func (s *TestScript) WithTargetDir(dir string) *TestScript {
	s.targetDir = dir
	return s
}

// Generate returns the shell script content.
func (s *TestScript) Generate() string {
	var lines []string

	lines = append(lines, "#!/bin/bash")
	lines = append(lines, "set -e")
	lines = append(lines, "")

	// pip install
	lines = append(lines, "pip install "+strings.Join(s.pipPackages, " "))
	lines = append(lines, "")

	// pytest command
	pytestCmd := "pytest " + strings.Join(s.pytestArgs, " ") + " " + s.targetDir
	lines = append(lines, pytestCmd)

	return strings.Join(lines, "\n")
}

// Save writes the script to test-notebooks.sh in the specified directory
// and makes it executable.
func (s *TestScript) Save(dir string) error {
	content := s.Generate()
	path := filepath.Join(dir, "test-notebooks.sh")

	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return err
	}

	return nil
}
