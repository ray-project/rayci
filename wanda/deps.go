package wanda

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolvedSpec contains a parsed and expanded spec along with its file path.
type ResolvedSpec struct {
	Spec *Spec
	Path string // original file path
}

// DepGraph represents a dependency graph of wanda specs.
type DepGraph struct {
	// specs maps expanded name to resolved spec
	specs map[string]*ResolvedSpec

	// order is the topological build order (dependencies first)
	order []string

	// root is the name of the root spec (the one requested to build)
	root string
}

// Order returns the build order (dependencies first, root last).
func (g *DepGraph) Order() []string {
	return g.order
}

// Root returns the name of the root spec.
func (g *DepGraph) Root() string {
	return g.root
}

// Specs returns all resolved specs in the graph.
func (g *DepGraph) Specs() map[string]*ResolvedSpec {
	return g.specs
}

// Get returns the resolved spec for the given name.
func (g *DepGraph) Get(name string) *ResolvedSpec {
	return g.specs[name]
}

// BuildDepGraph parses a spec and all its dependencies, returning a dependency graph
// with specs in topological build order.
func BuildDepGraph(specPath string, lookup lookupFunc) (*DepGraph, error) {
	g := &DepGraph{
		specs: make(map[string]*ResolvedSpec),
	}

	if err := g.loadSpec(specPath, lookup); err != nil {
		return nil, fmt.Errorf("load root spec: %w", err)
	}

	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return nil, fmt.Errorf("abs path for root: %w", err)
	}
	for name, rs := range g.specs {
		rsAbs, _ := filepath.Abs(rs.Path)
		if rsAbs == absPath {
			g.root = name
			break
		}
	}

	if err := g.topoSort(); err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}

	return g, nil
}

func (g *DepGraph) loadSpec(specPath string, lookup lookupFunc) error {
	spec, err := ParseSpecFile(specPath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", specPath, err)
	}
	spec = spec.expandVar(lookup)

	if err := checkUnexpandedVars(spec, specPath); err != nil {
		return err
	}

	if _, exists := g.specs[spec.Name]; exists {
		return nil
	}

	g.specs[spec.Name] = &ResolvedSpec{
		Spec: spec,
		Path: specPath,
	}

	specDir := filepath.Dir(specPath)
	for _, depPath := range spec.Deps {
		fullDepPath := depPath
		if !filepath.IsAbs(depPath) {
			fullDepPath = filepath.Join(specDir, depPath)
		}
		if err := g.loadSpec(fullDepPath, lookup); err != nil {
			return fmt.Errorf("load dep %s: %w", depPath, err)
		}
	}

	return nil
}

// localDeps extracts @-prefixed dependency names from a spec's Froms.
func localDeps(spec *Spec) []string {
	var deps []string
	for _, from := range spec.Froms {
		if strings.HasPrefix(from, "@") {
			deps = append(deps, strings.TrimPrefix(from, "@"))
		}
	}
	return deps
}

// topoSort performs topological sort using Kahn's algorithm.
func (g *DepGraph) topoSort() error {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for name := range g.specs {
		inDegree[name] = 0
	}

	for name, rs := range g.specs {
		for _, depName := range localDeps(rs.Spec) {
			if _, exists := g.specs[depName]; exists {
				inDegree[name]++
				dependents[depName] = append(dependents[depName], name)
			}
		}
	}

	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, dependent := range dependents[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(order) != len(g.specs) {
		var cycleNodes []string
		for name, degree := range inDegree {
			if degree > 0 {
				cycleNodes = append(cycleNodes, name)
			}
		}
		return fmt.Errorf("dependency cycle detected involving: %v", cycleNodes)
	}

	g.order = order
	return nil
}

// ValidateDeps checks that all @-prefixed references in Froms have corresponding
// entries in the deps list (i.e., they're in the graph).
func (g *DepGraph) ValidateDeps() error {
	for name, rs := range g.specs {
		for _, depName := range localDeps(rs.Spec) {
			if _, exists := g.specs[depName]; !exists {
				return fmt.Errorf(
					"spec %q references @%s in froms, but no dep provides image %q",
					name, depName, depName,
				)
			}
		}
	}
	return nil
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
	for _, s := range spec.Deps {
		if vars := findUnexpandedVars(s); len(vars) > 0 {
			missing = append(missing, vars...)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var unique []string
	for _, v := range missing {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}

	if len(unique) == 1 {
		return fmt.Errorf("%s: environment variable %s is not set", specPath, unique[0])
	}
	return fmt.Errorf("%s: environment variables not set: %s", specPath, strings.Join(unique, ", "))
}

// findUnexpandedVars finds $VAR patterns in a string that were not expanded.
func findUnexpandedVars(s string) []string {
	var vars []string
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) {
			// Skip $$
			if s[i+1] == '$' {
				i++
				continue
			}
			// Find the variable name
			j := i + 1
			for j < len(s) {
				c := s[j]
				if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' {
					j++
					continue
				}
				if c >= '0' && c <= '9' && j > i+1 {
					j++
					continue
				}
				break
			}
			if j > i+1 {
				vars = append(vars, s[i:j])
			}
			i = j - 1
		}
	}
	return vars
}
