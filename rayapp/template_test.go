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
			BuildID: "anyscaleray2340-py311",
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
			name: "both set",
			env: &ClusterEnv{
				BuildID:  "anyscaleray2440",
				ImageURI: "anyscale/ray:2.44.0",
			},
			wantErr:     true,
			errContains: "exactly one",
		},
		{
			name: "BYOD",
			env: &ClusterEnv{
				BYOD: &ClusterEnvBYOD{
					DockerImage: "cr.ray.io/ray:2340-py311",
					RayVersion:  "2.34.0",
				},
			},
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
			name: "BYOD containerfile only",
			env: &ClusterEnv{
				BYOD: &ClusterEnvBYOD{ContainerFile: "Dockerfile", RayVersion: "2.34.0"},
			},
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
		t.Fatal(
			"want error for cluster_env defined but empty (no build_id, image_uri, or byod), got nil",
		)
	}
	if !strings.Contains(err.Error(), "build_id or image_uri") &&
		!strings.Contains(err.Error(), "byod") {
		t.Errorf("error %q should mention build_id/image_uri or byod", err.Error())
	}
}

func TestReadTemplates_missingClusterEnv(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "BUILD.yaml")
	yaml := strings.Join([]string{
		"- name: no-cluster-env",
		"  dir: x",
		"  emoji: 📋",
		"  title: No cluster env",
		"  compute_config: {}",
	}, "\n")
	if err := os.WriteFile(f, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := readTemplates(f)
	if err == nil {
		t.Fatal("want error when cluster_env is missing, got nil")
	}
	if !strings.Contains(err.Error(), "cluster_env is required") {
		t.Errorf("error %q should contain %q", err.Error(), "cluster_env is required")
	}
}

func TestValidateClusterEnv(t *testing.T) {
	tests := []struct {
		name        string
		env         *ClusterEnv
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil env",
			env:         nil,
			wantErr:     true,
			errContains: "cluster_env is required",
		},
		{
			name: "BYOD docker_image and ray_version",
			env: &ClusterEnv{
				BYOD: &ClusterEnvBYOD{
					DockerImage: "cr.ray.io/ray:2340-py311",
					RayVersion:  "2.34.0",
				},
			},
			wantErr: false,
		},
		{
			name: "BYOD containerfile and ray_version",
			env: &ClusterEnv{
				BYOD: &ClusterEnvBYOD{ContainerFile: "Dockerfile", RayVersion: "2.34.0"},
			},
			wantErr: false,
		},
		{
			name: "BYOD both docker_image and containerfile",
			env: &ClusterEnv{
				BYOD: &ClusterEnvBYOD{
					DockerImage:   "img",
					ContainerFile: "Dockerfile",
					RayVersion:    "2.34.0",
				},
			},
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
			name:    "build_id only",
			env:     &ClusterEnv{BuildID: "anyscaleray2340-py311"},
			wantErr: false,
		},
		{
			name:    "image_uri only",
			env:     &ClusterEnv{ImageURI: "anyscale/ray:2.34.0-py311"},
			wantErr: false,
		},
		{
			name: "build_id and image_uri both set",
			env: &ClusterEnv{
				BuildID:  "anyscaleray2340-py311",
				ImageURI: "anyscale/ray:2.34.0-py311",
			},
			wantErr:     true,
			errContains: "exactly one",
		},
		{
			name:        "neither build_id nor image_uri",
			env:         &ClusterEnv{},
			wantErr:     true,
			errContains: "build_id or image_uri",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClusterEnv(tt.env)
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
				if !strings.Contains(err.Error(), "docker_image") &&
					!strings.Contains(err.Error(), "containerfile") {
					t.Errorf("error %q should mention docker_image or containerfile", err.Error())
				}
			case "byod missing ray_version":
				if !strings.Contains(err.Error(), "ray_version") {
					t.Errorf("error %q should mention ray_version requirement", err.Error())
				}
			case "byod both docker_image and containerfile":
				if !strings.Contains(err.Error(), "exactly one") &&
					!strings.Contains(err.Error(), "not both") {
					t.Errorf(
						"error %q should mention exactly one of docker_image or containerfile, not both",
						err.Error(),
					)
				}
			}
		})
	}
}

func TestConvertBuildIDToImageURI(t *testing.T) {
	tests := []struct {
		name           string
		buildID        string
		wantImageURI   string
		wantRayVersion string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid build ID with suffix",
			buildID:        "anyscaleray2441-py312-cu128",
			wantImageURI:   "anyscale/ray:2.44.1-py312-cu128",
			wantRayVersion: "2.44.1",
		},
		{
			name:           "valid build ID without suffix",
			buildID:        "anyscaleray2440",
			wantImageURI:   "anyscale/ray:2.44.0",
			wantRayVersion: "2.44.0",
		},
		{
			name:           "valid build ID with only python suffix",
			buildID:        "anyscaleray2350-py311",
			wantImageURI:   "anyscale/ray:2.35.0-py311",
			wantRayVersion: "2.35.0",
		},
		{
			name:           "valid build ID version 3",
			buildID:        "anyscaleray3001-py312",
			wantImageURI:   "anyscale/ray:3.00.1-py312",
			wantRayVersion: "3.00.1",
		},
		{
			name:           "valid build ID ray-llm",
			buildID:        "anyscalerayllm2441-py312-cu128",
			wantImageURI:   "anyscale/ray-llm:2.44.1-py312-cu128",
			wantRayVersion: "2.44.1",
		},
		{
			name:           "valid build ID ray-ml",
			buildID:        "anyscalerayml2440-py311",
			wantImageURI:   "anyscale/ray-ml:2.44.0-py311",
			wantRayVersion: "2.44.0",
		},
		{
			name:        "invalid prefix",
			buildID:     "rayimage2441-py312",
			wantErr:     true,
			errContains: "must start with",
		},
		{
			name:           "unknown image type used as image name",
			buildID:        "anyscalerayfoo2441-py312",
			wantImageURI:   "anyscale/rayfoo:2.44.1-py312",
			wantRayVersion: "2.44.1",
		},
		{
			name:        "version too short",
			buildID:     "anyscaleray123",
			wantErr:     true,
			errContains: "major(1 digit)",
		},
		{
			name:        "empty build ID",
			buildID:     "",
			wantErr:     true,
			errContains: "must start with",
		},
		{
			name:        "only prefix",
			buildID:     "anyscaleray",
			wantErr:     true,
			errContains: "major(1 digit)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imageURI, rayVersion, err := convertBuildIDToImageURI(tt.buildID)

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

func TestConvertImageURIToBuildID(t *testing.T) {
	tests := []struct {
		name        string
		imageURI    string
		wantBuildID string
	}{
		{
			name:        "image URI with suffix",
			imageURI:    "anyscale/ray:2.44.1-py312-cu128",
			wantBuildID: "anyscaleray2441-py312-cu128",
		},
		{
			name:        "image URI without suffix",
			imageURI:    "anyscale/ray:2.44.0",
			wantBuildID: "anyscaleray2440",
		},
		{
			name:        "image URI ray-llm",
			imageURI:    "anyscale/ray-llm:2.44.1-py312-cu128",
			wantBuildID: "anyscaleray-llm2441-py312-cu128",
		},
		{
			name:        "any URI is slugified",
			imageURI:    "other/ray:2.44.1-py312",
			wantBuildID: "otherray2441-py312",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildID, err := convertImageURIToBuildID(tt.imageURI)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if buildID != tt.wantBuildID {
				t.Errorf("buildID = %q, want %q", buildID, tt.wantBuildID)
			}
		})
	}
}
