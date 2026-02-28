package raycilint

import (
	"strings"
	"testing"
)

func TestCmdPrsize_MissingFlags(t *testing.T) {
	err := cmdPrsize(newConfig(), nil)
	if err == nil {
		t.Fatal("cmdPrsize() = nil, want error for missing flags")
	}
}

func TestCmdPrsize_InvalidFlag(t *testing.T) {
	err := cmdPrsize(newConfig(), []string{"-bogus"})
	if err == nil {
		t.Fatal("cmdPrsize() = nil, want error for invalid flag")
	}
}

func TestRunPrsize_NoThresholds(t *testing.T) {
	cfg := newConfig()
	g := &gitClient{}
	if err := runPrsize(cfg, "master", "feature", g); err != nil {
		t.Fatalf("runPrsize() error: %v", err)
	}
}

func TestRunPrsize_ExceedsThreshold(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "// line")
	}
	h.writeFile("big.go", strings.Join(lines, "\n")+"\n")
	h.commitAll("add big file")
	h.git("push", "origin", "feature")

	cfg := h.makeConfig(100, 0, nil)

	g := &gitClient{workDir: h.WorkDir}
	if err := runPrsize(cfg, "master", "feature", g); err == nil {
		t.Fatal("runPrsize() = nil, want error for exceeded threshold")
	}
}

func TestRunPrsize_UnderThreshold(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.writeFile("small.go", "package main\n")
	h.commitAll("add small file")
	h.git("push", "origin", "feature")

	cfg := h.makeConfig(10000, 10000, nil)

	g := &gitClient{workDir: h.WorkDir}
	if err := runPrsize(cfg, "master", "feature", g); err != nil {
		t.Fatalf("runPrsize() error: %v", err)
	}
}

func TestRunPrsize_IgnoredFiles(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "// line")
	}
	h.writeFile("vendor/big.go", strings.Join(lines, "\n")+"\n")
	h.writeFile("src/small.go", "package main\n")
	h.commitAll("add files")
	h.git("push", "origin", "feature")

	cfg := h.makeConfig(100, 0, []string{"vendor/"})

	g := &gitClient{workDir: h.WorkDir}
	if err := runPrsize(cfg, "master", "feature", g); err != nil {
		t.Fatalf("runPrsize() error: %v (vendor/ should be ignored)", err)
	}
}
