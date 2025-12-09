package raycicmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestListChangedFiles_InvalidCommitRange(t *testing.T) {
	client := &RealGitClient{}

	tests := []struct {
		name        string
		commitRange string
	}{
		{"no separator", "abc123"},
		{"wrong separator", "abc123..def456"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ListChangedFiles("main", tt.commitRange)
			if err == nil {
				t.Error(
					"ListChangedFiles() expected error for invalid commit range",
				)
			}
		})
	}
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestListChangedFiles_Integration(t *testing.T) {
	// Set up bare origin and working directory
	origin := t.TempDir()
	workDir := t.TempDir()

	// Initialize bare repo
	runGitCommand(t, origin, "init", "--bare")

	// Initialize working repo
	runGitCommand(t, workDir, "init")
	runGitCommand(t, workDir, "config", "user.email", "test@test.com")
	runGitCommand(t, workDir, "config", "user.name", "Test")
	runGitCommand(t, workDir, "remote", "add", "origin", origin)

	// Create initial commit on master
	readmePath := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# README\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitCommand(t, workDir, "add", "README.md")
	runGitCommand(t, workDir, "commit", "-m", "initial commit")
	runGitCommand(t, workDir, "push", "origin", "master")

	// Create a PR branch with changes
	runGitCommand(t, workDir, "checkout", "-b", "feature-branch")

	// Add some files
	changedFiles := []string{"src/main.go", "src/util.go", "docs/readme.txt"}
	for _, f := range changedFiles {
		fullPath := filepath.Join(workDir, f)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("content\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	runGitCommand(t, workDir, "add", ".")
	runGitCommand(t, workDir, "commit", "-m", "add files")

	// Get the commit hash
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workDir
	commitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("get commit: %v", err)
	}
	commit := strings.TrimSpace(string(commitBytes))

	// Test ListChangedFiles
	client := &RealGitClient{WorkDir: workDir}
	commitRange := "origin/master..." + commit

	files, err := client.ListChangedFiles("master", commitRange)
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	// Verify we got the expected files
	if len(files) != len(changedFiles) {
		t.Errorf(
			"got %d files, want %d: %v",
			len(files),
			len(changedFiles),
			files,
		)
	}

	for _, want := range changedFiles {
		found := false
		for _, got := range files {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing file %s in result %v", want, files)
		}
	}
}

func TestListChangedFiles_EmptyBaseBranch(t *testing.T) {
	// Set up bare origin and working directory
	origin := t.TempDir()
	workDir := t.TempDir()

	// Initialize bare repo
	runGitCommand(t, origin, "init", "--bare")

	// Initialize working repo
	runGitCommand(t, workDir, "init")
	runGitCommand(t, workDir, "config", "user.email", "test@test.com")
	runGitCommand(t, workDir, "config", "user.name", "Test")
	runGitCommand(t, workDir, "remote", "add", "origin", origin)

	// Create initial commit
	readmePath := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# README\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitCommand(t, workDir, "add", "README.md")
	runGitCommand(t, workDir, "commit", "-m", "initial commit")

	// Get the base commit
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workDir
	baseCommitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("get base commit: %v", err)
	}
	baseCommit := strings.TrimSpace(string(baseCommitBytes))

	// Add another file
	newFilePath := filepath.Join(workDir, "new_file.txt")
	if err := os.WriteFile(newFilePath, []byte("new content\n"), 0644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	runGitCommand(t, workDir, "add", "new_file.txt")
	runGitCommand(t, workDir, "commit", "-m", "add new file")

	// Get the new commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workDir
	newCommitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("get new commit: %v", err)
	}
	newCommit := strings.TrimSpace(string(newCommitBytes))

	// Test ListChangedFiles with empty baseBranch (skips fetch)
	client := &RealGitClient{WorkDir: workDir}
	commitRange := baseCommit + "..." + newCommit

	files, err := client.ListChangedFiles("", commitRange)
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	// Should have exactly one file
	if len(files) != 1 || files[0] != "new_file.txt" {
		t.Errorf("got files %v, want [new_file.txt]", files)
	}
}
