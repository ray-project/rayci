package wanda

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// depGraph represents a dependency graph of wanda specs.
type depGraph struct {
	// Specs maps name to expanded spec (all discovered specs).
	Specs map[string]*resolvedSpec

	// Order is the topological build order (dependencies first, root last).
	// Contains only specs reachable from Root.
	Order []string

	// Root is the name of the root spec (the one requested to build).
	Root string

	// discovery state (unexported)
	baseDir    string     // base directory for resolving spec paths
	specDirs   []string   // directories to scan for specs (relative to baseDir)
	lookup     lookupFunc // lookup function for environment variables
	namePrefix string     // prefix identifying wanda-built images (e.g. "cr.ray.io/rayproject/")
}

// resolvedSpec contains a parsed and expanded spec along with its file path.
type resolvedSpec struct {
	Spec *Spec
	Path string
}

// buildDepGraph parses a spec and all its dependencies, returning a dependency graph
// with specs in deterministic topological build order.
// namePrefix identifies wanda-built images (e.g. "cr.ray.io/rayproject/").
// wandaSpecsFile is the path to the file listing spec directories.
func buildDepGraph(specPath string, lookup lookupFunc, namePrefix, wandaSpecsFile string) (*depGraph, error) {
	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}

	specDirs, err := readWandaSpecs(wandaSpecsFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", wandaSpecsFile, err)
	}

	// Spec directories are relative to the directory containing wandaSpecsFile.
	baseDir := filepath.Dir(wandaSpecsFile)

	g := &depGraph{
		Specs:      make(map[string]*resolvedSpec),
		baseDir:    baseDir,
		specDirs:   specDirs,
		lookup:     lookup,
		namePrefix: namePrefix,
	}

	spec, err := parseSpecFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("parse root spec: %w", err)
	}
	expanded := spec.expandVar(lookup)
	if err := checkUnexpandedVars(expanded, absPath); err != nil {
		return nil, fmt.Errorf("check root spec: %w", err)
	}
	g.Root = expanded.Name
	g.Specs[expanded.Name] = &resolvedSpec{Spec: expanded, Path: absPath}

	// Lazily discover and load all dependencies.
	if err := g.loadDeps(expanded.Name); err != nil {
		return nil, fmt.Errorf("load deps: %w", err)
	}

	// Filter to reachable specs and sort.
	reachable := g.filterReachable()
	if err := g.topoSort(reachable); err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}

	return g, nil
}

// loadDeps recursively loads dependencies for a spec into g.Specs.
func (g *depGraph) loadDeps(name string) error {
	spec := g.Specs[name]
	if spec == nil {
		return nil
	}

	for _, depName := range localDeps(spec.Spec, g.namePrefix) {
		if _, exists := g.Specs[depName]; exists {
			continue
		}
		if err := g.discover(); err != nil {
			return err
		}
		if _, exists := g.Specs[depName]; !exists {
			log.Printf("warning: no wanda spec found for %s%s, treating as external image", g.namePrefix, depName)
			continue
		}
		if err := g.loadDeps(depName); err != nil {
			return err
		}
	}
	return nil
}

// discover lazily discovers all specs from specDirs into g.Specs.
func (g *depGraph) discover() error {
	if g.specDirs == nil {
		return nil
	}

	for _, dir := range g.specDirs {
		searchRoot := dir
		if !filepath.IsAbs(dir) {
			searchRoot = filepath.Join(g.baseDir, dir)
		}
		idx, err := discoverSpecs(searchRoot, g.lookup)
		if err != nil {
			return fmt.Errorf("discover specs in %s: %w", dir, err)
		}
		for k, v := range idx {
			if _, exists := g.Specs[k]; !exists {
				g.Specs[k] = v
			}
		}
	}

	// Clear the specDirs list to prevent re-discovery.
	g.specDirs = nil
	return nil
}

// filterReachable returns the set of spec names reachable from Root.
func (g *depGraph) filterReachable() map[string]bool {
	reachable := make(map[string]bool)
	var visit func(name string)
	visit = func(name string) {
		if reachable[name] {
			return
		}
		spec := g.Specs[name]
		if spec == nil {
			return
		}
		reachable[name] = true
		for _, depName := range localDeps(spec.Spec, g.namePrefix) {
			visit(depName)
		}
	}
	visit(g.Root)
	return reachable
}

// localDeps extracts wanda-built dependency names from a spec's Froms.
// Images with the namePrefix (e.g. "cr.ray.io/rayproject/foo") are identified
// as wanda-built and the name portion ("foo") is returned.
// Tags are stripped (e.g. "foo:v1.0" becomes "foo") since the spec index uses names without tags.
// If namePrefix is empty, no dependencies are detected.
func localDeps(spec *Spec, namePrefix string) []string {
	if namePrefix == "" {
		return nil
	}
	var deps []string
	for _, from := range spec.Froms {
		if strings.HasPrefix(from, namePrefix) {
			depName := strings.TrimPrefix(from, namePrefix)
			// Strip tag if present.
			if idx := strings.Index(depName, ":"); idx != -1 {
				depName = depName[:idx]
			}
			deps = append(deps, depName)
		}
	}
	return deps
}

// topoSort performs a deterministic topological sort layer by layer.
// Each layer contains nodes whose dependencies are all in previous layers.
// Within each layer, nodes are sorted alphabetically.
// Only specs in the reachable set are included in the sort.
func (g *depGraph) topoSort(reachable map[string]bool) error {
	inDegree := make(map[string]int, len(reachable))
	dependents := make(map[string][]string, len(reachable))

	// Init all reachable nodes.
	for name := range reachable {
		inDegree[name] = 0
	}

	// Build edges: dep -> dependent (only for reachable specs)
	for name := range reachable {
		rs := g.Specs[name]
		for _, depName := range localDeps(rs.Spec, g.namePrefix) {
			if reachable[depName] {
				inDegree[name]++
				dependents[depName] = append(dependents[depName], name)
			}
		}
	}

	var order []string
	for len(order) < len(reachable) {
		// Collect all nodes with in-degree 0.
		var layer []string
		for name, degree := range inDegree {
			if degree == 0 {
				layer = append(layer, name)
			}
		}

		if len(layer) == 0 {
			// Cycle detected - no nodes with in-degree 0 but graph not empty.
			var cycleNodes []string
			for name, degree := range inDegree {
				if degree > 0 {
					cycleNodes = append(cycleNodes, name)
				}
			}
			sort.Strings(cycleNodes)
			return fmt.Errorf("dependency cycle detected involving: %v", cycleNodes)
		}

		// Sort layer alphabetically for deterministic order.
		sort.Strings(layer)

		// Add layer to order and remove from graph.
		for _, name := range layer {
			order = append(order, name)
			delete(inDegree, name)
			for _, dependent := range dependents[name] {
				inDegree[dependent]--
			}
		}
	}

	g.Order = order
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
