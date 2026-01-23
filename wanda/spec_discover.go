package wanda

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// specIndex maps spec names to resolved specs.
type specIndex map[string]*resolvedSpec

// discoverSpecs scans searchRoot for *.wanda.yaml files and builds a name index.
// Specs are expanded using the provided lookup function.
// Returns an error if two specs expand to the same name.
func discoverSpecs(searchRoot string, lookup lookupFunc) (specIndex, error) {
	index := make(specIndex)
	conflicts := make(map[string]map[string]struct{})
	var minConflictName string

	err := filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".wanda.yaml") {
			return nil
		}

		spec, err := parseSpecFile(path)
		if err != nil {
			return nil // skip unparseable files
		}

		// Expand the name using env lookup and index it.
		expanded := spec.expandVar(lookup)
		name := expanded.Name

		// Skip specs with unexpanded variables.
		if strings.Contains(name, "$") {
			return nil
		}

		if existing, exists := index[name]; exists && existing.Path != path {
			// Record conflict.
			m := conflicts[name]
			if m == nil {
				m = make(map[string]struct{}, 2)
				conflicts[name] = m
			}
			m[existing.Path] = struct{}{}
			m[path] = struct{}{}
			if minConflictName == "" || name < minConflictName {
				minConflictName = name
			}
			return nil
		}
		index[name] = &resolvedSpec{Path: path, Spec: expanded}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", searchRoot, err)
	}

	if len(conflicts) > 0 {
		// Use the smallest conflicting name (deterministic) without sorting all names.
		name := minConflictName
		m := conflicts[name]
		paths := make([]string, 0, len(m))
		for p := range m {
			paths = append(paths, p)
		}
		sort.Strings(paths) // optional, just for stable error output
		return nil, fmt.Errorf("multiple specs have name %q: %s", name, strings.Join(paths, ", "))
	}

	return index, nil
}

// readWandaSpecs reads the wandaSpecsFile.
// Each non-empty line (after trimming whitespace) is treated as a directory path.
// Lines starting with # are comments and are ignored.
// Returns nil (no directories) if the file doesn't exist.
func readWandaSpecs(wandaSpecsFile string) ([]string, error) {
	data, err := os.ReadFile(wandaSpecsFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dirs = append(dirs, line)
	}
	return dirs, nil
}

// checkUnexpandedVars checks if a spec has any unexpanded environment variables
// and returns a helpful error message if so.
func checkUnexpandedVars(spec *Spec, specPath string) error {
	var missing []string

	if vars := findUnexpandedVars(spec.Name); len(vars) > 0 {
		missing = append(missing, vars...)
	}
	for _, s := range spec.Froms {
		if vars := findUnexpandedVars(s); len(vars) > 0 {
			missing = append(missing, vars...)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	uniqueMap := make(map[string]struct{})
	for _, v := range missing {
		uniqueMap[v] = struct{}{}
	}
	unique := make([]string, 0, len(uniqueMap))
	for v := range uniqueMap {
		unique = append(unique, v)
	}
	sort.Strings(unique)

	if len(unique) == 1 {
		return fmt.Errorf("%s: environment variable %s is not set", specPath, unique[0])
	}
	return fmt.Errorf("%s: environment variables not set: %s", specPath, strings.Join(unique, ", "))
}

// findUnexpandedVars finds $VAR patterns in a string that were not expanded.
func findUnexpandedVars(s string) []string {
	var vars []string
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '$' && i+1 < len(runes) {
			// Skip $$
			if runes[i+1] == '$' {
				i++
				continue
			}
			// Find the variable name
			j := i + 1
			for j < len(runes) {
				c := runes[j]
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
					j++
					continue
				}
				if (c >= '0' && c <= '9') && j > i+1 {
					j++
					continue
				}
				break
			}
			if j > i+1 {
				vars = append(vars, string(runes[i:j]))
			}
			i = j - 1
		}
	}
	return vars
}
