package wanda

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Artifact defines a file or directory to extract from the built image.
type Artifact struct {
	// Src is the path inside the container to extract.
	// Can be a file, directory, or glob pattern (e.g., "/build/*.whl").
	// Supports variable expansion.
	Src string `yaml:"src"`

	// Dst is the destination path on the host filesystem.
	// If it ends with "/", src is copied into the directory.
	// Otherwise, src is copied as the file/directory named dst.
	// Relative paths are relative to ArtifactsDir.
	Dst string `yaml:"dst"`

	// Optional marks this artifact as best-effort.
	// If true, extraction failure will be logged but won't fail the build.
	Optional bool `yaml:"optional,omitempty"`
}

// Spec is a specification for a container image.
type Spec struct {
	Name string `yaml:"name,omitempty"`

	Tags []string `yaml:"tags"`

	// Inputs
	Froms      []string `yaml:"froms"`
	Srcs       []string `yaml:"srcs,omitempty"`
	Dockerfile string   `yaml:"dockerfile"`

	BuildArgs []string `yaml:"build_args,omitempty"`

	// BuildHintArgs are build args which values do not participate
	// in cache input compute. The value of these build args should not
	// change the output of the build.
	BuildHintArgs []string `yaml:"build_hint_args,omitempty"`

	// DisableCaching disables use of caching.
	DisableCaching bool `yaml:"disable_caching,omitempty"`

	// Artifacts defines files and directories to extract from the built image.
	Artifacts []*Artifact `yaml:"artifacts,omitempty"`
}

func parseSpecFile(f string) (*Spec, error) {
	bs, err := os.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	spec := new(Spec)
	dec := yaml.NewDecoder(bytes.NewReader(bs))
	dec.KnownFields(true)
	if err := dec.Decode(spec); err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}

	return spec, nil
}

type lookupFunc func(string) (string, bool)

func expandVar(s string, lookup lookupFunc) string {
	buf := new(bytes.Buffer)
	inName := false
	nameStart := 0

	replace := func(k string) string {
		if v, ok := lookup(k); ok {
			return v
		}
		return "$" + k
	}

	for i, r := range s {
		if !inName {
			if r == '$' {
				inName = true
				nameStart = i + 1
			} else {
				buf.WriteRune(r)
			}
		} else {
			if r == '$' {
				if nameStart == i {
					// Name is empty, this is $$
					buf.WriteRune('$')
					inName = false
					continue
				}
			}
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				continue
			}
			if r == '_' {
				continue
			}
			if r >= '0' && r <= '9' && i > nameStart {
				continue
			}

			buf.WriteString(replace(s[nameStart:i]))
			if r == '$' {
				// keep inName as true
				nameStart = i + 1
			} else {
				inName = false
				buf.WriteRune(r)
			}
		}
	}

	if inName {
		buf.WriteString(replace(s[nameStart:]))
	}

	return buf.String()
}

func stringsExpandVar(slice []string, lookup lookupFunc) []string {
	if len(slice) == 0 {
		return nil
	}
	result := make([]string, len(slice))
	for i, s := range slice {
		result[i] = expandVar(s, lookup)
	}
	return result
}

func artifactsExpandVar(artifacts []*Artifact, lookup lookupFunc) []*Artifact {
	if len(artifacts) == 0 {
		return nil
	}
	result := make([]*Artifact, len(artifacts))
	for i, a := range artifacts {
		result[i] = &Artifact{
			Src:      expandVar(a.Src, lookup),
			Dst:      expandVar(a.Dst, lookup),
			Optional: a.Optional,
		}
	}
	return result
}

func (s *Spec) expandVar(lookup lookupFunc) *Spec {
	result := new(Spec)

	result.Name = expandVar(s.Name, lookup)
	result.Tags = stringsExpandVar(s.Tags, lookup)
	result.Froms = stringsExpandVar(s.Froms, lookup)
	result.Srcs = stringsExpandVar(s.Srcs, lookup)
	result.Dockerfile = expandVar(s.Dockerfile, lookup)
	result.BuildArgs = stringsExpandVar(s.BuildArgs, lookup)
	result.BuildHintArgs = stringsExpandVar(s.BuildHintArgs, lookup)
	result.DisableCaching = s.DisableCaching
	result.Artifacts = artifactsExpandVar(s.Artifacts, lookup)

	return result
}
