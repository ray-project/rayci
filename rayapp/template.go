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
	ContainerFile string `yaml:"containerfile" json:"containerfile"`
	DockerImage   string `yaml:"docker_image" json:"docker_image"`
	RayVersion    string `yaml:"ray_version,omitempty" json:"ray_version,omitempty"`
}

// ClusterEnv is the cluster environment for Anyscale clusters.
type ClusterEnv struct {
	BuildID  string `yaml:"build_id,omitempty" json:"build_id,omitempty"`
	ImageURI string `yaml:"image_uri,omitempty" json:"image_uri,omitempty"`

	// BYOD is the cluster environment for bring-your-own-docker clusters.
	BYOD *ClusterEnvBYOD `yaml:"byod,omitempty" json:"byod,omitempty"`
}

// Template defines the definition of a workspace template.
type Template struct {
	Name string `yaml:"name" json:"name"`
	Dir  string `yaml:"dir" json:"dir"`

	Emoji       string `yaml:"emoji" json:"emoji"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	ClusterEnv *ClusterEnv `yaml:"cluster_env" json:"cluster_env"`

	// A map of files for different compute platforms.
	ComputeConfig map[string]string `yaml:"compute_config" json:"compute_config"`
}

func convertBuildIdToImageURI(buildId string) (string, string, error) {
	// Convert build ID like "anyscaleray2441-py312-cu128" to "anyscale/ray:2.44.1-py312-cu128"
	const prefix = "anyscaleray"
	if !strings.HasPrefix(buildId, prefix) {
		return "", "", fmt.Errorf("build ID must start with %q: %s", prefix, buildId)
	}

	// Remove the prefix to get "2441-py312-cu128"
	remainder := strings.TrimPrefix(buildId, prefix)

	// Find the first hyphen to separate version from suffix
	hyphenIdx := strings.Index(remainder, "-")
	var versionStr, suffix string
	if hyphenIdx == -1 {
		versionStr = remainder
		suffix = ""
	} else {
		versionStr = remainder[:hyphenIdx]
		suffix = remainder[hyphenIdx:] // includes the hyphen
	}

	// Parse version: "2441" -> "2.44.1"
	// Format: major (1 digit), minor (2 digits), patch (1+ digits)
	buildIDVersionRe := regexp.MustCompile(`^(\d)(\d{2})(\d+)$`)
	matches := buildIDVersionRe.FindStringSubmatch(versionStr)
	if matches == nil {
		return "", "", fmt.Errorf("version string must match major(1 digit).minor(2 digits).patch(1+ digits): %s", versionStr)
	}
	major, minor, patch := matches[1], matches[2], matches[3]

	return fmt.Sprintf("anyscale/ray:%s.%s.%s%s", major, minor, patch, suffix), fmt.Sprintf("%s.%s.%s", major, minor, patch), nil
}

// convertImageURIToBuildID converts an image URI like "anyscale/ray:2.44.1-py312-cu128"
// to a build ID like "anyscaleray2441-py312-cu128" and returns the ray version "2.44.1".
func convertImageURIToBuildID(imageURI string) (buildID, rayVersion string, err error) {
	const prefix = "anyscale/ray:"
	if !strings.HasPrefix(imageURI, prefix) {
		return "", "", fmt.Errorf("image URI must start with %q: %s", prefix, imageURI)
	}
	tag := strings.TrimPrefix(imageURI, prefix)
	hyphenIdx := strings.Index(tag, "-")
	var versionStr, suffix string
	if hyphenIdx == -1 {
		versionStr = tag
		suffix = ""
	} else {
		versionStr = tag[:hyphenIdx]
		suffix = tag[hyphenIdx:]
	}
	// Require exactly 3 parts: major (1 digit).minor (2 digits).patch (1+ digits)
	imageURIVersionRe := regexp.MustCompile(`^(\d)\.(\d{2})\.(\d+)$`)
	matches := imageURIVersionRe.FindStringSubmatch(versionStr)
	if matches == nil {
		return "", "", fmt.Errorf("image URI version must match major(1 digit).minor(2 digits).patch(1+ digits): %s", versionStr)
	}
	major, minor, patch := matches[1], matches[2], matches[3]
	versionCompact := major + minor + patch
	return "anyscaleray" + versionCompact + suffix, versionStr, nil
}

// getImageURIAndRayVersionFromClusterEnv returns image URI and ray version from cluster env.
// It supports BYOD (docker_image + ray_version) or BuildID/ImageURI; when both BuildID and ImageURI are set, ImageURI is used.
func getImageURIAndRayVersionFromClusterEnv(env *ClusterEnv) (imageURI, rayVersion string, err error) {
	if env == nil {
		return "", "", fmt.Errorf("cluster_env is required")
	}
	if env.BYOD != nil {
		if env.BYOD.ContainerFile != "" {
			return "", "", fmt.Errorf("cluster_env byod: containerfile is used via --containerfile; image URI not applicable")
		}
		if strings.TrimSpace(env.BYOD.DockerImage) == "" || strings.TrimSpace(env.BYOD.RayVersion) == "" {
			return "", "", fmt.Errorf("cluster_env byod: both docker_image and ray_version are required")
		}
		return env.BYOD.DockerImage, env.BYOD.RayVersion, nil
	}
	hasBuildID := strings.TrimSpace(env.BuildID) != ""
	hasImageURI := strings.TrimSpace(env.ImageURI) != ""
	switch {
	case !hasBuildID && !hasImageURI:
		return "", "", fmt.Errorf("cluster_env: specify build_id or image_uri, or byod with docker_image and ray_version")
	case hasImageURI:
		_, rayVersion, err := convertImageURIToBuildID(env.ImageURI)
		if err != nil {
			return "", "", err
		}
		return env.ImageURI, rayVersion, nil
	default:
		return convertBuildIdToImageURI(env.BuildID)
	}
}

// validateAndBuildClusterEnv validates ClusterEnv (BYOD or build_id/image_uri) and populates the missing one of BuildID or ImageURI when exactly one is set.
func validateAndBuildClusterEnv(env *ClusterEnv) error {
	if env == nil {
		return nil
	}
	hasBuildID := env.BuildID != ""
	hasImageURI := env.ImageURI != ""
	if env.BYOD != nil {
		hasDocker := env.BYOD.DockerImage != ""
		hasContainer := env.BYOD.ContainerFile != ""
		if hasDocker && hasContainer {
			return fmt.Errorf("cluster_env byod: specify exactly one of docker_image or containerfile, not both")
		}
		if !hasDocker && !hasContainer {
			return fmt.Errorf("cluster_env byod: specify one of docker_image or containerfile")
		}
		if env.BYOD.RayVersion == "" {
			return fmt.Errorf("cluster_env byod: ray_version is required")
		}
		return nil
	}
	if !hasBuildID && !hasImageURI {
		return fmt.Errorf("cluster_env: specify at least one of build_id or image_uri, or use byod with docker_image and ray_version")
	}
	if hasBuildID && hasImageURI {
		imageURIFromBuildID, _, err := convertBuildIdToImageURI(env.BuildID)
		if err != nil {
			return err
		}
		if imageURIFromBuildID != env.ImageURI {
			return fmt.Errorf("build_id and image_uri do not match: build_id %q implies image_uri %q", env.BuildID, imageURIFromBuildID)
		}
		return nil
	}
	if hasBuildID {
		imageURI, _, err := convertBuildIdToImageURI(env.BuildID)
		if err != nil {
			return err
		}
		env.ImageURI = imageURI
		return nil
	}
	buildID, _, err := convertImageURIToBuildID(env.ImageURI)
	if err != nil {
		return err
	}
	env.BuildID = buildID
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
		if err := validateAndBuildClusterEnv(tmpl.ClusterEnv); err != nil {
			return nil, fmt.Errorf("resolve cluster env for template %q: %w", tmpl.Name, err)
		}
	}
	return tmpls, nil
}
