package raycicmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReviewPolicy(t *testing.T) {
	tmp := t.TempDir()
	bkDir := filepath.Join(tmp, ".buildkite")
	if err := os.MkdirAll(bkDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := strings.Join([]string{
		"review:",
		"  max_additions: 300",
		"  max_deletions: 200",
		"  ignore:",
		"    - vendor/",
		"    - generated/",
	}, "\n")
	if err := os.WriteFile(filepath.Join(bkDir, "review-policy.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	policy, err := loadReviewPolicy(bkDir)
	if err != nil {
		t.Fatalf("loadReviewPolicy: %v", err)
	}
	if policy == nil {
		t.Fatal("loadReviewPolicy() = nil, want non-nil")
	}
	if policy.Review == nil {
		t.Fatal("Review = nil, want non-nil")
	}
	if got, want := policy.Review.MaxAdditions, 300; got != want {
		t.Errorf("MaxAdditions = %d, want %d", got, want)
	}
	if got, want := policy.Review.MaxDeletions, 200; got != want {
		t.Errorf("MaxDeletions = %d, want %d", got, want)
	}
	if got, want := len(policy.Review.Ignore), 2; got != want {
		t.Errorf("len(Ignore) = %d, want %d", got, want)
	}
}

func TestLoadReviewPolicy_NotFound(t *testing.T) {
	tmp := t.TempDir()

	policy, err := loadReviewPolicy(tmp)
	if err != nil {
		t.Fatalf("loadReviewPolicy: %v", err)
	}
	if policy != nil {
		t.Errorf("loadReviewPolicy() = %v, want nil", policy)
	}
}

func TestMakePolicyGroup_ExceedsThreshold(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	var lines []string
	for i := 0; i < 600; i++ {
		lines = append(lines, "// line")
	}
	h.writeFile("big.go", strings.Join(lines, "\n")+"\n")
	commit := h.commitAll("add big file")

	lister := &GitChangeLister{
		WorkDir:    h.WorkDir,
		BaseBranch: "master",
		Commit:     commit,
	}

	policy := &reviewPolicy{
		Review: &reviewConfig{
			MaxAdditions: 100,
			MaxDeletions: 0,
		},
	}

	g := makePolicyGroup(policy, lister, "runner_x")
	if g == nil {
		t.Fatal("makePolicyGroup() = nil, want non-nil")
	}
	if g.Group != "Policy" {
		t.Errorf("Group = %q, want %q", g.Group, "Policy")
	}

	step := g.Steps[0].(map[string]any)
	if _, ok := step["soft_fail"]; !ok {
		t.Error("step missing soft_fail")
	}
	cmds, _ := step["command"].([]string)
	hasAnnotate := false
	for _, c := range cmds {
		if strings.Contains(c, "buildkite-agent annotate") {
			hasAnnotate = true
		}
	}
	if !hasAnnotate {
		t.Error("step commands missing buildkite-agent annotate")
	}
	agents, ok := step["agents"].(map[string]any)
	if !ok {
		t.Fatal("step missing agents")
	}
	if got, want := agents["queue"], "runner_x"; got != want {
		t.Errorf("agents queue = %v, want %v", got, want)
	}
}

func TestMakePolicyGroup_UnderThreshold(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	h.writeFile("small.go", "package main\n")
	commit := h.commitAll("add small file")

	lister := &GitChangeLister{
		WorkDir:    h.WorkDir,
		BaseBranch: "master",
		Commit:     commit,
	}

	policy := &reviewPolicy{
		Review: &reviewConfig{
			MaxAdditions: 10000,
			MaxDeletions: 10000,
		},
	}

	if g := makePolicyGroup(policy, lister, "runner_x"); g != nil {
		t.Errorf("makePolicyGroup() = %v, want nil", g)
	}
}

func TestMakePolicyGroup_IgnoredFiles(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	var lines []string
	for i := 0; i < 600; i++ {
		lines = append(lines, "// line")
	}
	h.writeFile("vendor/big.go", strings.Join(lines, "\n")+"\n")
	h.writeFile("src/small.go", "package main\n")
	commit := h.commitAll("add files")

	lister := &GitChangeLister{
		WorkDir:    h.WorkDir,
		BaseBranch: "master",
		Commit:     commit,
	}

	policy := &reviewPolicy{
		Review: &reviewConfig{
			MaxAdditions: 100,
			MaxDeletions: 0,
			Ignore:       []string{"vendor/"},
		},
	}

	if g := makePolicyGroup(policy, lister, "runner_x"); g != nil {
		t.Errorf("makePolicyGroup() = %v, want nil (vendor/ should be ignored)", g)
	}
}

func TestMakePolicyGroup_NilPolicy(t *testing.T) {
	if g := makePolicyGroup(nil, nil, ""); g != nil {
		t.Errorf("makePolicyGroup(nil) = %v, want nil", g)
	}

	policy := &reviewPolicy{}
	if g := makePolicyGroup(policy, nil, ""); g != nil {
		t.Errorf("makePolicyGroup(empty) = %v, want nil", g)
	}
}
