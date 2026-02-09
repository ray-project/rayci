package rayapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testBuildDotYaml = `
# Just for testing

- name: job-intro
  emoji: 🔰
  title: Intro to Jobs
  description: Introduction on how to use Anyscale Jobs
  dir: templates/intro-jobs
  cluster_env:
    build_id: anyscaleray2340-py311
  compute_config:
    GCP: configs/basic-single-node/gce.yaml
    AWS: configs/basic-single-node/aws.yaml

- name: job-intro-image-uri
  emoji: 📦
  title: Intro to Jobs (image URI)
  description: Same as job-intro but cluster_env uses image_uri
  dir: templates/intro-jobs
  cluster_env:
    image_uri: anyscale/ray:2.34.0-py311
  compute_config:
    GCP: configs/basic-single-node/gce.yaml
    AWS: configs/basic-single-node/aws.yaml

- name: workspace-intro
  emoji: 🔰
  title: Intro to Workspaces
  description: Introduction on how to use Anyscale Workspaces
  dir: templates/intro-workspaces
  cluster_env:
    byod:
      docker_image: cr.ray.io/ray:2340-py311
      ray_version: 2.34.0
  compute_config:
    GCP: configs/basic-single-node/gce.yaml
    AWS: configs/basic-single-node/aws.yaml

- name: no-cluster-env
  emoji: 📋
  title: No cluster env
  description: Template with no cluster_env (legal)
  dir: templates/no-env
  compute_config:
    GCP: configs/basic-single-node/gce.yaml
    AWS: configs/basic-single-node/aws.yaml
`

func TestReadTemplates(t *testing.T) {
	tmp := t.TempDir()

	f := filepath.Join(tmp, "BUILD.yaml")
	if err := os.WriteFile(f, []byte(testBuildDotYaml), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := readTemplates(f)
	if err != nil {
		t.Fatalf("readTemplates(%q): %v", f, err)
	}

	want := []*Template{{
		Name:        "job-intro",
		Emoji:       "🔰",
		Title:       "Intro to Jobs",
		Dir:         "templates/intro-jobs",
		Description: "Introduction on how to use Anyscale Jobs",
		ClusterEnv: &ClusterEnv{
			BuildID:  "anyscaleray2340-py311",
			ImageURI: "anyscale/ray:2.34.0-py311", // populated from build_id during read
		},
		ComputeConfig: map[string]string{
			"GCP": "configs/basic-single-node/gce.yaml",
			"AWS": "configs/basic-single-node/aws.yaml",
		},
	}, {
		Name:        "job-intro-image-uri",
		Emoji:       "📦",
		Title:       "Intro to Jobs (image URI)",
		Dir:         "templates/intro-jobs",
		Description: "Same as job-intro but cluster_env uses image_uri",
		ClusterEnv: &ClusterEnv{
			ImageURI: "anyscale/ray:2.34.0-py311",
			BuildID:  "anyscaleray2340-py311", // populated from image_uri during read
		},
		ComputeConfig: map[string]string{
			"GCP": "configs/basic-single-node/gce.yaml",
			"AWS": "configs/basic-single-node/aws.yaml",
		},
	}, {
		Name:        "workspace-intro",
		Emoji:       "🔰",
		Title:       "Intro to Workspaces",
		Dir:         "templates/intro-workspaces",
		Description: "Introduction on how to use Anyscale Workspaces",
		ClusterEnv: &ClusterEnv{
			BYOD: &ClusterEnvBYOD{
				DockerImage: "cr.ray.io/ray:2340-py311",
				RayVersion:  "2.34.0",
			},
		},
		ComputeConfig: map[string]string{
			"GCP": "configs/basic-single-node/gce.yaml",
			"AWS": "configs/basic-single-node/aws.yaml",
		},
	}, {
		Name:        "no-cluster-env",
		Emoji:       "📋",
		Title:       "No cluster env",
		Dir:         "templates/no-env",
		Description: "Template with no cluster_env (legal)",
		ClusterEnv:  nil,
		ComputeConfig: map[string]string{
			"GCP": "configs/basic-single-node/gce.yaml",
			"AWS": "configs/basic-single-node/aws.yaml",
		},
	}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("readTemplates(%q), got %+v, want %+v", f, got, want)
	}

	// Loopback with JSON encoding, and it should be the same.
	jsonBytes, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal templates: %v", err)
	}

	jsonFile := filepath.Join(tmp, "BUILD.json")
	if err := os.WriteFile(jsonFile, jsonBytes, 0o600); err != nil {
		t.Fatalf("write json file: %v", err)
	}

	jsonGot, err := readTemplates(jsonFile)
	if err != nil {
		t.Fatalf("read json file: %v", err)
	}
	if !reflect.DeepEqual(jsonGot, want) {
		t.Errorf("read json file, got %+v, want %+v", jsonGot, want)
	}
}

func TestReadTemplates_withError(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "BUILD.yaml")
	if _, err := readTemplates(f); err == nil {
		t.Error("want error, got nil")
	}
}

func TestGetImageURIAndRayVersionFromClusterEnv(t *testing.T) {
	tests := []struct {
		name           string
		env            *ClusterEnv
		wantImageURI   string
		wantRayVersion string
		wantErr        bool
		errContains    string
	}{
		{
			name:         "only BuildID",
			env:          &ClusterEnv{BuildID: "anyscaleray2441-py312-cu128"},
			wantImageURI: "anyscale/ray:2.44.1-py312-cu128", wantRayVersion: "2.44.1",
		},
		{
			name:         "only ImageURI",
			env:          &ClusterEnv{ImageURI: "anyscale/ray:2.44.1-py312-cu128"},
			wantImageURI: "anyscale/ray:2.44.1-py312-cu128", wantRayVersion: "2.44.1",
		},
		{
			name:           "both set",
			env:            &ClusterEnv{BuildID: "anyscaleray2440", ImageURI: "anyscale/ray:2.44.0"},
			wantImageURI:   "anyscale/ray:2.44.0",
			wantRayVersion: "2.44.0",
		},
		{
			name:           "BYOD",
			env:            &ClusterEnv{BYOD: &ClusterEnvBYOD{DockerImage: "cr.ray.io/ray:2340-py311", RayVersion: "2.34.0"}},
			wantImageURI:   "cr.ray.io/ray:2340-py311",
			wantRayVersion: "2.34.0",
		},
		{
			name:        "neither set",
			env:         &ClusterEnv{},
			wantErr:     true,
			errContains: "build_id or image_uri",
		},
		{
			name:        "nil env",
			env:         nil,
			wantErr:     true,
			errContains: "cluster_env is required",
		},
		{
			name:        "BYOD containerfile only",
			env:         &ClusterEnv{BYOD: &ClusterEnvBYOD{ContainerFile: "Dockerfile", RayVersion: "2.34.0"}},
			wantErr:     true,
			errContains: "containerfile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imageURI, rayVersion, err := getImageURIAndRayVersionFromClusterEnv(tt.env)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if imageURI != tt.wantImageURI {
				t.Errorf("imageURI = %q, want %q", imageURI, tt.wantImageURI)
			}
			if rayVersion != tt.wantRayVersion {
				t.Errorf("rayVersion = %q, want %q", rayVersion, tt.wantRayVersion)
			}
		})
	}
}

func TestReadTemplates_buildIDImageURIMismatch(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "BUILD.yaml")
	yaml := strings.Join([]string{
		"- name: bad",
		"  dir: x",
		"  cluster_env:",
		"    build_id: anyscaleray2340-py311",
		"    image_uri: anyscale/ray:2.44.1-py312-cu128",
		"  compute_config: {}",
	}, "\n")
	if err := os.WriteFile(f, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := readTemplates(f)
	if err == nil {
		t.Fatal("want error for mismatched build_id and image_uri, got nil")
	}
	if !strings.Contains(err.Error(), "do not match") {
		t.Errorf("error %q should contain %q", err.Error(), "do not match")
	}
}

func TestReadTemplates_emptyClusterEnv(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "BUILD.yaml")
	yaml := strings.Join([]string{
		"- name: empty-env",
		"  dir: x",
		"  cluster_env: {}",
		"  compute_config: {}",
	}, "\n")
	if err := os.WriteFile(f, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := readTemplates(f)
	if err == nil {
		t.Fatal("want error for cluster_env defined but empty (no build_id, image_uri, or byod), got nil")
	}
	if !strings.Contains(err.Error(), "build_id or image_uri") && !strings.Contains(err.Error(), "byod") {
		t.Errorf("error %q should mention build_id/image_uri or byod", err.Error())
	}
}

func TestValidateAndBuildClusterEnv(t *testing.T) {
	tests := []struct {
		name        string
		env         *ClusterEnv
		wantErr     bool
		errContains string
		// optional: after success, expected populated fields (env may be mutated)
		wantImageURI string
		wantBuildID  string
	}{
		{
			name:    "nil env",
			env:     nil,
			wantErr: false,
		},
		{
			name:    "BYOD docker_image and ray_version",
			env:     &ClusterEnv{BYOD: &ClusterEnvBYOD{DockerImage: "cr.ray.io/ray:2340-py311", RayVersion: "2.34.0"}},
			wantErr: false,
		},
		{
			name:    "BYOD containerfile and ray_version",
			env:     &ClusterEnv{BYOD: &ClusterEnvBYOD{ContainerFile: "Dockerfile", RayVersion: "2.34.0"}},
			wantErr: false,
		},
		{
			name:        "BYOD both docker_image and containerfile",
			env:         &ClusterEnv{BYOD: &ClusterEnvBYOD{DockerImage: "img", ContainerFile: "Dockerfile", RayVersion: "2.34.0"}},
			wantErr:     true,
			errContains: "exactly one",
		},
		{
			name:        "BYOD neither docker_image nor containerfile",
			env:         &ClusterEnv{BYOD: &ClusterEnvBYOD{RayVersion: "2.34.0"}},
			wantErr:     true,
			errContains: "docker_image or containerfile",
		},
		{
			name:        "BYOD missing ray_version",
			env:         &ClusterEnv{BYOD: &ClusterEnvBYOD{DockerImage: "cr.ray.io/ray:2340"}},
			wantErr:     true,
			errContains: "ray_version",
		},
		{
			name:         "build_id only populates image_uri",
			env:          &ClusterEnv{BuildID: "anyscaleray2340-py311"},
			wantErr:      false,
			wantImageURI: "anyscale/ray:2.34.0-py311",
		},
		{
			name:        "image_uri only populates build_id",
			env:         &ClusterEnv{ImageURI: "anyscale/ray:2.34.0-py311"},
			wantErr:     false,
			wantBuildID: "anyscaleray2340-py311",
		},
		{
			name:    "build_id and image_uri matching",
			env:     &ClusterEnv{BuildID: "anyscaleray2340-py311", ImageURI: "anyscale/ray:2.34.0-py311"},
			wantErr: false,
		},
		{
			name:        "build_id and image_uri mismatching",
			env:         &ClusterEnv{BuildID: "anyscaleray2340-py311", ImageURI: "anyscale/ray:2.44.1-py312-cu128"},
			wantErr:     true,
			errContains: "do not match",
		},
		{
			name:        "neither build_id nor image_uri",
			env:         &ClusterEnv{},
			wantErr:     true,
			errContains: "build_id or image_uri",
		},
		{
			name:        "invalid build_id format",
			env:         &ClusterEnv{BuildID: "bad-format"},
			wantErr:     true,
			errContains: "anyscaleray",
		},
		{
			name:        "invalid image_uri format",
			env:         &ClusterEnv{ImageURI: "wrong/format:tag"},
			wantErr:     true,
			errContains: "image URI",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAndBuildClusterEnv(tt.env)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantImageURI != "" && (tt.env == nil || tt.env.ImageURI != tt.wantImageURI) {
				got := ""
				if tt.env != nil {
					got = tt.env.ImageURI
				}
				t.Errorf("ImageURI = %q, want %q", got, tt.wantImageURI)
			}
			if tt.wantBuildID != "" && (tt.env == nil || tt.env.BuildID != tt.wantBuildID) {
				got := ""
				if tt.env != nil {
					got = tt.env.BuildID
				}
				t.Errorf("BuildID = %q, want %q", got, tt.wantBuildID)
			}
		})
	}
}

func TestReadTemplates_byodIncomplete(t *testing.T) {
	for _, tt := range []struct {
		name string
		yaml string
	}{
		{"byod missing docker_image", strings.Join([]string{
			"- name: x",
			"  dir: x",
			"  cluster_env:",
			"    byod:",
			"      ray_version: 2.34.0",
			"  compute_config: {}",
		}, "\n")},
		{"byod missing ray_version", strings.Join([]string{
			"- name: x",
			"  dir: x",
			"  cluster_env:",
			"    byod:",
			"      docker_image: cr.ray.io/ray:2340",
			"  compute_config: {}",
		}, "\n")},
		{"byod both docker_image and containerfile", strings.Join([]string{
			"- name: x",
			"  dir: x",
			"  cluster_env:",
			"    byod:",
			"      docker_image: cr.ray.io/ray:2340",
			"      containerfile: Dockerfile",
			"      ray_version: 2.34.0",
			"  compute_config: {}",
		}, "\n")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			f := filepath.Join(tmp, "BUILD.yaml")
			if err := os.WriteFile(f, []byte(tt.yaml), 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}
			_, err := readTemplates(f)
			if err == nil {
				t.Fatal("want error for incomplete byod, got nil")
			}
			switch tt.name {
			case "byod missing docker_image":
				if !strings.Contains(err.Error(), "docker_image") && !strings.Contains(err.Error(), "containerfile") {
					t.Errorf("error %q should mention docker_image or containerfile", err.Error())
				}
			case "byod both docker_image and containerfile":
				if !strings.Contains(err.Error(), "exactly one") && !strings.Contains(err.Error(), "not both") {
					t.Errorf("error %q should mention exactly one of docker_image or containerfile, not both", err.Error())
				}
			default:
				if !strings.Contains(err.Error(), "ray_version") {
					t.Errorf("error %q should mention ray_version requirement", err.Error())
				}
			}
		})
	}
}
