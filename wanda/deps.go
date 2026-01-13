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
	// Specs maps expanded name to resolved spec.
	Specs map[string]*resolvedSpec

	// Order is the topological build order (dependencies first, root last).
	Order []string

	// Root is the name of the root spec (the one requested to build).
	Root string

	// discovery state (unexported)
	baseDir    string    // base directory for resolving relative spec dirs
	specDirs   []string  // directories to scan for specs (relative to baseDir)
	index      specIndex // lazily populated name -> path index
	lookup     lookupFunc
	namePrefix string // prefix identifying wanda-built images (e.g. "cr.ray.io/rayproject/")
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

	if err := g.loadSpec(absPath, true /* isRoot */); err != nil {
		return nil, fmt.Errorf("load root spec: %w", err)
	}

	if err := g.topoSort(); err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}

	return g, nil
}

func (g *depGraph) loadSpec(specPath string, isRoot bool) error {
	spec, err := parseSpecFile(specPath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", specPath, err)
	}
	spec = spec.expandVar(g.lookup)

	if err := checkUnexpandedVars(spec, specPath); err != nil {
		return err
	}

	// Root is simply the spec we were asked to build.
	if isRoot {
		g.Root = spec.Name
	}

	// If we've already loaded this spec by name, don't re-walk it.
	if _, exists := g.Specs[spec.Name]; exists {
		return nil
	}

	g.Specs[spec.Name] = &resolvedSpec{
		Spec: spec,
		Path: specPath,
	}

	// Resolve wanda-built dependencies via discovery.
	for _, depName := range localDeps(spec, g.namePrefix) {
		if _, exists := g.Specs[depName]; exists {
			continue
		}
		if err := g.discoverAndLoad(depName); err != nil {
			return fmt.Errorf("resolve %s%s: %w", g.namePrefix, depName, err)
		}
	}

	return nil
}

func (g *depGraph) discoverAndLoad(name string) error {
	// Lazy index initialization.
	if g.index == nil {
		g.index = make(specIndex)
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
				g.index[k] = v
			}
		}
	}

	specPath, ok := g.index[name]
	if !ok {
		// No wanda spec found - treat as external image.
		log.Printf("warning: no wanda spec found for %s%s, treating as external image", g.namePrefix, name)
		return nil
	}

	return g.loadSpec(specPath, false /* isRoot */)
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
func (g *depGraph) topoSort() error {
	inDegree := make(map[string]int, len(g.Specs))
	dependents := make(map[string][]string, len(g.Specs))

	// Init all nodes.
	for name := range g.Specs {
		inDegree[name] = 0
	}

	// Build edges: dep -> dependent
	for name, rs := range g.Specs {
		for _, depName := range localDeps(rs.Spec, g.namePrefix) {
			if _, exists := g.Specs[depName]; exists {
				inDegree[name]++
				dependents[depName] = append(dependents[depName], name)
			}
		}
	}

	var order []string
	for len(order) < len(g.Specs) {
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

func (g *depGraph) validateDeps() error {
	names := make([]string, 0, len(g.Specs))
	for name := range g.Specs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		rs := g.Specs[name]
		for _, depName := range localDeps(rs.Spec, g.namePrefix) {
			if _, exists := g.Specs[depName]; !exists {
				// If dep wasn't in discovery index, it's external - skip validation
				if g.index != nil {
					if _, inIndex := g.index[depName]; !inIndex {
						continue
					}
				}
				return fmt.Errorf(
					"spec %q references %s%s in froms, but no spec provides image %q",
					name, g.namePrefix, depName, depName,
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

// specIndex maps expanded spec names to their file paths.
type specIndex map[string]string

// discoverSpecs scans searchRoot for *.wanda.yaml files and builds a name index.
// Names are expanded using the provided lookup function.
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

		if existing, exists := index[name]; exists && existing != path {
			// Record conflict.
			m := conflicts[name]
			if m == nil {
				m = make(map[string]struct{}, 2)
				conflicts[name] = m
			}
			m[existing] = struct{}{}
			m[path] = struct{}{}
			if minConflictName == "" || name < minConflictName {
				minConflictName = name
			}
			return nil
		}
		index[name] = path
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
