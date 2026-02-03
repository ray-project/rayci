package wanda

import (
	"fmt"
	"log"
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
