package rayapp

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// ClusterEnvBYOD is the cluster environment for BYOD clusters.
type ClusterEnvBYOD struct {
	ContainerFile string `yaml:"containerfile"         json:"containerfile"`
	DockerImage   string `yaml:"docker_image"          json:"docker_image"`
	RayVersion    string `yaml:"ray_version,omitempty" json:"ray_version,omitempty"`
}

// ClusterEnv is the cluster environment for Anyscale clusters.
type ClusterEnv struct {
	BuildID  string `yaml:"build_id,omitempty"  json:"build_id,omitempty"`
	ImageURI string `yaml:"image_uri,omitempty" json:"image_uri,omitempty"`

	// BYOD is the cluster environment for bring-your-own-docker clusters.
	BYOD *ClusterEnvBYOD `yaml:"byod,omitempty" json:"byod,omitempty"`
}

// TestConfig defines test configuration for a template.
type TestConfig struct {
	TimeoutInSec int    `yaml:"timeout_in_sec,omitempty" json:"timeout_in_sec,omitempty"`
	TestsPath    string `yaml:"tests_path,omitempty"     json:"tests_path,omitempty"`
	Command      string `yaml:"command"                  json:"command"`
}

// Template defines the definition of a workspace template.
type Template struct {
	Name string `yaml:"name" json:"name"`
	Dir  string `yaml:"dir"  json:"dir"`

	Emoji       string `yaml:"emoji"                 json:"emoji"`
	Title       string `yaml:"title"                 json:"title"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	ClusterEnv *ClusterEnv `yaml:"cluster_env" json:"cluster_env"`

	// A map of files for different compute platforms.
	ComputeConfig map[string]string `yaml:"compute_config" json:"compute_config"`

	// Test configuration for the template (optional).
	Test *TestConfig `yaml:"test,omitempty" json:"test,omitempty"`
}

// Find first version-like digit sequence in build ID remainder (unanchored).
var buildIDVersionFindRe = regexp.MustCompile(`(\d)(\d{2})(\d+)`)

// buildIDToImageName maps build ID slugified image-type (after "anyscale") to image name for URI.
var buildIDToImageName = map[string]string{
	"ray":    "ray",
	"rayllm": "ray-llm",
	"rayml":  "ray-ml",
}

func convertBuildIDToImageURI(buildID string) (string, string, error) {
	const prefix = "anyscale"
	if !strings.HasPrefix(buildID, prefix) {
		return "", "", fmt.Errorf("build ID must start with %q: %s", prefix, buildID)
	}

	remainder := strings.TrimPrefix(buildID, prefix)
	locs := buildIDVersionFindRe.FindStringSubmatchIndex(remainder)
	if locs == nil {
		err := fmt.Errorf(
			"version string must match major(1 digit), minor(2 digits), patch(1+) in build ID: %s",
			buildID,
		)
		return "", "", err
	}
	slugifiedImageType := remainder[:locs[0]]
	suffix := remainder[locs[1]:]

	imageName, ok := buildIDToImageName[slugifiedImageType]
	if !ok {
		imageName = slugifiedImageType
	}

	major := remainder[locs[2]:locs[3]]
	minor := remainder[locs[4]:locs[5]]
	patch := remainder[locs[6]:locs[7]]

	imageURI := fmt.Sprintf("anyscale/%s:%s.%s.%s%s", imageName, major, minor, patch, suffix)
	rayVersion := fmt.Sprintf("%s.%s.%s", major, minor, patch)
	return imageURI, rayVersion, nil
}

var slugifyRemoveRe = regexp.MustCompile(`[^\w\s-]+`)
var slugifyCollapseRe = regexp.MustCompile(`[-\s]+`)

// slugify converts a string to a slug (Django-style): normalize to ASCII, keep only alphanumerics/
// underscores/hyphens/spaces, strip, then collapse spaces and hyphens to single hyphens.
// Code adopted from here https://github.com/django/django/blob/master/django/utils/text.py
func slugify(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 128 {
			return r
		}
		return -1
	}, value)
	value = slugifyRemoveRe.ReplaceAllString(value, "")
	value = strings.TrimSpace(value)
	return slugifyCollapseRe.ReplaceAllString(value, "-")
}

// convertImageURIToBuildID returns the slugified image URI as the build ID.
func convertImageURIToBuildID(imageURI string) (buildID string, err error) {
	return slugify(imageURI), nil
}

var imageURIVersionRe = regexp.MustCompile(`(\d)\.(\d{2})\.(\d+)`)

// extractRayVersionFromImageURI returns the ray version from the image URI.
func extractRayVersionFromImageURI(imageURI string) (rayVersion string, err error) {
	matches := imageURIVersionRe.FindStringSubmatch(imageURI)
	if matches == nil {
		err := fmt.Errorf(
			"image URI version must match major(1 digit).minor(2 digits).patch(1+ digits): %s",
			imageURI,
		)
		return "", err
	}
	major, minor, patch := matches[1], matches[2], matches[3]
	return fmt.Sprintf("%s.%s.%s", major, minor, patch), nil
}

// getImageURIAndRayVersionFromClusterEnv returns image URI and ray version from cluster env.
// It supports BYOD (docker_image + ray_version) or BuildID/ImageURI.
func getImageURIAndRayVersionFromClusterEnv(env *ClusterEnv) (string, string, error) {
	if env.BYOD != nil {
		if env.BYOD.ContainerFile != "" {
			err := fmt.Errorf(
				"cluster_env byod: containerfile is used via --containerfile; image URI not applicable",
			)
			return "", "", err
		}
		if strings.TrimSpace(env.BYOD.DockerImage) == "" ||
			strings.TrimSpace(env.BYOD.RayVersion) == "" {
			err := fmt.Errorf("cluster_env byod: both docker_image and ray_version are required")
			return "", "", err
		}
		return env.BYOD.DockerImage, env.BYOD.RayVersion, nil
	}
	hasBuildID := strings.TrimSpace(env.BuildID) != ""
	hasImageURI := strings.TrimSpace(env.ImageURI) != ""
	switch {
	case hasBuildID && hasImageURI:
		err := fmt.Errorf("cluster_env: specify exactly one of build_id or image_uri, not both")
		return "", "", err
	case !hasBuildID && !hasImageURI:
		err := fmt.Errorf(
			"cluster_env: specify build_id or image_uri, or byod with docker_image and ray_version",
		)
		return "", "", err
	case hasImageURI:
		rayVersion, err := extractRayVersionFromImageURI(env.ImageURI)
		if err != nil {
			return "", "", fmt.Errorf("failed to extract ray version from image URI: %w", err)
		}
		return env.ImageURI, rayVersion, nil
	default:
		imageURI, rayVersion, err := convertBuildIDToImageURI(env.BuildID)
		if err != nil {
			return "", "", fmt.Errorf("failed to convert build ID to image URI: %w", err)
		}
		return imageURI, rayVersion, nil
	}
}

// validateTestConfig returns an error if TestConfig is invalid; otherwise nil.
// Nil test config is valid (test configuration is optional).
func validateTestConfig(test *TestConfig) error {
	if test == nil {
		return nil
	}
	if strings.TrimSpace(test.Command) == "" {
		return fmt.Errorf("test.command is required")
	}
	if test.TimeoutInSec < 0 {
		return fmt.Errorf("test.timeout_in_sec must be non-negative")
	}
	return nil
}

// validateClusterEnv returns an error if ClusterEnv is nil or invalid (BYOD or build_id/image_uri);
// otherwise nil.
func validateClusterEnv(env *ClusterEnv) error {
	if env == nil {
		return fmt.Errorf("cluster_env is required")
	}
	if env.BYOD != nil {
		hasDocker := strings.TrimSpace(env.BYOD.DockerImage) != ""
		hasContainer := strings.TrimSpace(env.BYOD.ContainerFile) != ""
		if hasDocker && hasContainer {
			return fmt.Errorf(
				"cluster_env byod: specify exactly one of docker_image or containerfile, not both",
			)
		}
		if !hasDocker && !hasContainer {
			return fmt.Errorf("cluster_env byod: specify one of docker_image or containerfile")
		}
		if strings.TrimSpace(env.BYOD.RayVersion) == "" {
			return fmt.Errorf("cluster_env byod: ray_version is required")
		}
		return nil
	}
	hasBuildID := strings.TrimSpace(env.BuildID) != ""
	hasImageURI := strings.TrimSpace(env.ImageURI) != ""
	if (!hasBuildID && !hasImageURI) || (hasBuildID && hasImageURI) {
		return fmt.Errorf("cluster_env: specify exactly one of build_id or image_uri, not both")
	}
	return nil
}

func readTemplates(yamlFile string) ([]*Template, error) {
	var tmpls []*Template

	bs, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", yamlFile, err)
	}
	if err := yaml.Unmarshal(bs, &tmpls); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	for _, tmpl := range tmpls {
		if err := validateClusterEnv(tmpl.ClusterEnv); err != nil {
			return nil, fmt.Errorf("validate cluster env for template %q: %w", tmpl.Name, err)
		}
		if err := validateTestConfig(tmpl.Test); err != nil {
			return nil, fmt.Errorf("validate test config for template %q: %w", tmpl.Name, err)
		}
		if tmpl.Test != nil && tmpl.Test.TimeoutInSec == 0 {
			tmpl.Test.TimeoutInSec = 3600
		}
	}
	return tmpls, nil
}
