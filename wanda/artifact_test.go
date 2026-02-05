package wanda

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifact_Validate(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		dst     string
		wantErr string
	}{
		{"valid absolute src", "/app/file.txt", "output.txt", ""},
		{"valid glob src", "/*.whl", "wheels/", ""},
		{"valid nested dst", "/app/file.txt", "subdir/output.txt", ""},
		{"relative src rejected", "relative/path.txt", "output.txt", "must be absolute"},
		{"absolute dst rejected", "/app/file.txt", "/absolute/path.txt", "must be relative"},
		{"dst with .. rejected", "/app/file.txt", "../escape.txt", "cannot escape"},
		{"dst with nested .. rejected", "/app/file.txt", "subdir/../../escape.txt", "cannot escape"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Artifact{Src: tt.src, Dst: tt.dst}
			err := a.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestArtifact_HasGlob(t *testing.T) {
	tests := []struct {
		src  string
		want bool
	}{
		{"/app/file.txt", false},
		{"/app/*.txt", true},
		{"/app/file?.txt", true},
		{"/app/[abc].txt", true},
		{"/*.whl", true},
		{"/path/to/dir/", false},
	}

	for _, tt := range tests {
		a := &Artifact{Src: tt.src}
		got := a.HasGlob()
		if got != tt.want {
			t.Errorf("Artifact{Src: %q}.HasGlob() = %v, want %v", tt.src, got, tt.want)
		}
	}
}

func TestArtifact_ResolveSrcs(t *testing.T) {
	containerFiles := []string{
		"/app.whl",
		"/other.whl",
		"/readme.txt",
		"/etc/config",
		"/build/output.whl",
	}

	tests := []struct {
		src  string
		want []string
	}{
		{"/*.whl", []string{"/app.whl", "/other.whl"}},
		{"/*.txt", []string{"/readme.txt"}},
		{"/build/*.whl", []string{"/build/output.whl"}},
		{"/*.exe", nil},
		{"/app.whl", []string{"/app.whl"}},         // non-glob returns src directly
		{"/nonexistent", []string{"/nonexistent"}}, // non-glob doesn't check existence
	}

	for _, tt := range tests {
		a := &Artifact{Src: tt.src}
		got := a.ResolveSrcs(containerFiles)
		if len(got) != len(tt.want) {
			t.Errorf("Artifact{Src: %q}.ResolveSrcs() = %v, want %v", tt.src, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("Artifact{Src: %q}.ResolveSrcs()[%d] = %q, want %q", tt.src, i, got[i], tt.want[i])
			}
		}
	}
}

func TestArtifact_ResolveDst(t *testing.T) {
	artifactsDir := t.TempDir()

	tests := []struct {
		name    string
		dst     string
		want    string
		wantErr string
	}{
		{"simple file", "output.txt", filepath.Join(artifactsDir, "output.txt"), ""},
		{"subdirectory", "wheels/file.whl", filepath.Join(artifactsDir, "wheels/file.whl"), ""},
		{"trailing slash", "wheels/", filepath.Join(artifactsDir, "wheels"), ""},
		{"absolute rejected", "/absolute/path.txt", "", "must be relative"},
		{"escape rejected", "../escape.txt", "", "escapes artifacts"},
		{"nested escape rejected", "sub/../../escape.txt", "", "escapes artifacts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Artifact{Dst: tt.dst}
			got, err := a.ResolveDst(artifactsDir)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ResolveDst() unexpected error: %v", err)
				} else if got != tt.want {
					t.Errorf("ResolveDst() = %q, want %q", got, tt.want)
				}
			} else {
				if err == nil {
					t.Errorf("ResolveDst() expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ResolveDst() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}
