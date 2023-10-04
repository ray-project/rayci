package wanda

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Spec is a specification for a container image.
type Spec struct {
	Name string `yaml:"name,omitempty"`

	Tags []string `yaml:"tags"`

	// Inputs
	Froms      []string `yaml:"froms"`
	Srcs       []string `yaml:"srcs,omitempty"`
	Dockerfile string   `yaml:"dockerfile"`

	BuildArgs []string `yaml:"build_args,omitempty"`

	// Wastefully include everything on work directory into the image.
	CopyEverything bool `yaml:"copy_everything,omitempty"`
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

func stringsExpanVar(slice []string, lookup lookupFunc) []string {
	if len(slice) == 0 {
		return nil
	}
	result := make([]string, len(slice))
	for i, s := range slice {
		result[i] = expandVar(s, lookup)
	}
	return result
}

func (s *Spec) expandVar(lookup lookupFunc) *Spec {
	result := new(Spec)

	result.Name = expandVar(s.Name, lookup)
	result.Tags = stringsExpanVar(s.Tags, lookup)
	result.Froms = stringsExpanVar(s.Froms, lookup)
	result.Srcs = stringsExpanVar(s.Srcs, lookup)
	result.Dockerfile = expandVar(s.Dockerfile, lookup)
	result.BuildArgs = stringsExpanVar(s.BuildArgs, lookup)

	return result
}
