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

	// Params declares allowed values for environment variables used in
	// templated fields, currently only `name` and `froms` are supported.
	// Keys are variable names (without $), values are lists of allowed values.
	Params map[string][]string `yaml:"params,omitempty"`

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
	result.Params = s.Params // contains literal values, not expanded.
	result.Tags = stringsExpanVar(s.Tags, lookup)
	result.Froms = stringsExpanVar(s.Froms, lookup)
	result.Srcs = stringsExpanVar(s.Srcs, lookup)
	result.Dockerfile = expandVar(s.Dockerfile, lookup)
	result.BuildArgs = stringsExpanVar(s.BuildArgs, lookup)
	result.BuildHintArgs = stringsExpanVar(s.BuildHintArgs, lookup)

	return result
}

// extractVarNames extracts all variable names from a string.
// Uses the same parsing rules as expandVar.
func extractVarNames(s string) []string {
	var vars []string
	inName := false
	nameStart := 0

	for i, r := range s {
		if !inName {
			if r == '$' {
				inName = true
				nameStart = i + 1
			}
		} else {
			if r == '$' {
				if nameStart == i {
					// $$ escape sequence
					inName = false
					continue
				}
			}
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_' {
				continue
			}
			if r >= '0' && r <= '9' && i > nameStart {
				continue
			}

			if i > nameStart {
				vars = append(vars, s[nameStart:i])
			}
			if r == '$' {
				nameStart = i + 1
			} else {
				inName = false
			}
		}
	}

	if inName && len(s) > nameStart {
		vars = append(vars, s[nameStart:])
	}

	return vars
}

// expandVarWithParams returns all possible expanded values for a string
// using declared params. Generates the cartesian product when multiple
// variables are present. Variables without params are preserved as $VARNAME.
func expandVarWithParams(s string, params map[string][]string) []string {
	vars := extractVarNames(s)
	if len(vars) == 0 {
		return []string{s}
	}

	// Find which vars have params
	var paramVars []string
	for _, v := range vars {
		if _, ok := params[v]; ok {
			paramVars = append(paramVars, v)
		}
	}

	if len(paramVars) == 0 {
		return []string{s}
	}

	// Generate all combinations
	combinations := []map[string]string{{}}
	for _, v := range paramVars {
		var newCombinations []map[string]string
		for _, combo := range combinations {
			for _, val := range params[v] {
				newCombo := make(map[string]string, len(combo)+1)
				for k, v := range combo {
					newCombo[k] = v
				}
				newCombo[v] = val
				newCombinations = append(newCombinations, newCombo)
			}
		}
		combinations = newCombinations
	}

	// Expand each combination
	var results []string
	for _, combo := range combinations {
		expanded := expandVar(s, func(k string) (string, bool) {
			v, ok := combo[k]
			return v, ok
		})
		results = append(results, expanded)
	}

	return results
}

// ExpandedNames returns all possible names based on declared params.
func (s *Spec) ExpandedNames() []string {
	return expandVarWithParams(s.Name, s.Params)
}

// ExpandedFroms returns all possible froms based on declared params.
// Results are deduplicated.
func (s *Spec) ExpandedFroms() []string {
	seen := make(map[string]bool)
	var results []string
	for _, from := range s.Froms {
		for _, expanded := range expandVarWithParams(from, s.Params) {
			if !seen[expanded] {
				seen[expanded] = true
				results = append(results, expanded)
			}
		}
	}
	return results
}

// ValidateParams validates that environment variable values match declared params.
func (s *Spec) ValidateParams(lookup lookupFunc) error {
	for varName, allowed := range s.Params {
		val, ok := lookup(varName)
		if !ok {
			continue // Unset vars are handled by expandVar
		}

		valid := false
		for _, a := range allowed {
			if val == a {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf(
				"env var %s has value %q, but params only allow: %v",
				varName, val, allowed,
			)
		}
	}
	return nil
}

// UnexpandedVars returns all unexpanded variable names found in Name and Froms.
func (s *Spec) UnexpandedVars() []string {
	var vars []string
	vars = append(vars, extractVarNames(s.Name)...)
	for _, from := range s.Froms {
		vars = append(vars, extractVarNames(from)...)
	}
	return vars
}

// tryFullyExpand attempts to fully expand a string using the lookup function.
// Returns the expanded string and true if successful, or the original string
// and false if unexpanded variables remain.
func tryFullyExpand(s string, lookup lookupFunc) (string, bool) {
	if len(extractVarNames(s)) == 0 {
		return s, true
	}
	expanded := expandVar(s, lookup)
	return expanded, len(extractVarNames(expanded)) == 0
}
