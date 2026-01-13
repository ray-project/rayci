package wanda

import (
	"container/heap"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
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
	searchRoot string    // repo root or spec dir for discovery
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
func buildDepGraph(specPath string, lookup lookupFunc, namePrefix string) (*depGraph, error) {
	absPath, err := filepath.Abs(specPath)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}
	specDir := filepath.Dir(absPath)

	g := &depGraph{
		Specs:      make(map[string]*resolvedSpec),
		searchRoot: findRepoRoot(specDir),
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

	if err := spec.ValidateParams(g.lookup); err != nil {
		return fmt.Errorf("%s: %w", specPath, err)
	}

	spec = spec.expandVar(g.lookup)

	if err := checkUnexpandedVars(spec, specPath, spec.Params); err != nil {
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
		var err error
		g.index, err = discoverSpecs(g.searchRoot, g.lookup)
		if err != nil {
			return fmt.Errorf("discover specs: %w", err)
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
// If namePrefix is empty, no dependencies are detected.
func localDeps(spec *Spec, namePrefix string) []string {
	if namePrefix == "" {
		return nil
	}
	var deps []string
	for _, from := range spec.Froms {
		if strings.HasPrefix(from, namePrefix) {
			deps = append(deps, strings.TrimPrefix(from, namePrefix))
		}
	}
	return deps
}

type stringHeap []string

func (h stringHeap) Len() int           { return len(h) }
func (h stringHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h stringHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *stringHeap) Push(x any)        { *h = append(*h, x.(string)) }
func (h *stringHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// topoSort performs a deterministic topological sort using Kahn's algorithm.
// When multiple nodes are available, it picks lexicographically smallest first.
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

	// Stable traversal over dependents.
	for k := range dependents {
		sort.Strings(dependents[k])
	}

	// Stable ready-set: min-heap by name.
	h := &stringHeap{}
	heap.Init(h)
	for name, degree := range inDegree {
		if degree == 0 {
			heap.Push(h, name)
		}
	}

	order := make([]string, 0, len(g.Specs))
	for h.Len() > 0 {
		current := heap.Pop(h).(string)
		order = append(order, current)

		for _, dependent := range dependents[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				heap.Push(h, dependent)
			}
		}
	}

	if len(order) != len(g.Specs) {
		cycleNodes := make([]string, 0)
		for name, degree := range inDegree {
			if degree > 0 {
				cycleNodes = append(cycleNodes, name)
			}
		}
		sort.Strings(cycleNodes) // stable error output too
		return fmt.Errorf("dependency cycle detected involving: %v", cycleNodes)
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
// and returns a helpful error message. If params are declared for a missing var,
// the valid values are included in the error message.
func checkUnexpandedVars(spec *Spec, specPath string, params map[string][]string) error {
	vars := spec.UnexpandedVars()
	if len(vars) == 0 {
		return nil
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, v := range vars {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}

	// Build error message with param hints where available
	var parts []string
	for _, v := range unique {
		if allowed, ok := params[v]; ok && len(allowed) > 0 {
			parts = append(parts, fmt.Sprintf("$%s (valid values: %s)", v, strings.Join(allowed, ", ")))
		} else {
			parts = append(parts, "$"+v)
		}
	}

	if len(parts) == 1 {
		return fmt.Errorf("%s: environment variable %s is not set", specPath, parts[0])
	}
	return fmt.Errorf("%s: environment variables not set: %s", specPath, strings.Join(parts, "; "))
}

// findRepoRoot walks up from startDir looking for a .git directory.
// Returns the repo root path, or startDir if no .git is found.
func findRepoRoot(startDir string) string {
	dir := startDir
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir
		}
		dir = parent
	}
}

// specIndex maps expanded spec names to their file paths.
type specIndex map[string]string

type discovered struct {
	name string
	path string
	// skipped indicates “ignore this file” (unparseable / unexpanded vars / etc.)
	skipped bool
}

// discoverSpecs scans searchRoot for *.wanda.yaml files and builds a name index.
// Names are expanded using declared params first, then the lookup function.
// Specs with params will have all param combinations indexed.
// Returns an error if two specs expand to the same name.
func discoverSpecs(searchRoot string, lookup lookupFunc) (specIndex, error) {
	index := make(specIndex)

	// For conflicts we want name -> unique paths
	conflicts := make(map[string]map[string]struct{})
	var minConflictName string // track smallest conflict name without sorting everything

	pathsCh := make(chan string, 256)
	outCh := make(chan discovered, 256)

	// 1) Workers parse concurrently
	workers := runtime.GOMAXPROCS(0) // or runtime.NumCPU()
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for path := range pathsCh {
				spec, err := parseSpecFile(path)
				if err != nil {
					outCh <- discovered{skipped: true}
					continue
				}

				// Use params to enumerate all possible names.
				// For vars without params, try env lookup.
				sentAny := false
				for _, name := range spec.ExpandedNames() {
					if expanded, ok := tryFullyExpand(name, lookup); ok {
						outCh <- discovered{name: expanded, path: path}
						sentAny = true
					}
				}
				if !sentAny {
					outCh <- discovered{skipped: true}
				}
			}
		}()
	}

	// 2) Close outCh when workers finish
	go func() {
		wg.Wait()
		close(outCh)
	}()

	// 3) Walk directory (single goroutine) and feed candidate files
	walkErr := filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if d.IsDir() {
			name := d.Name()
			// Skip common non-source directories
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			// Skip hidden and underscore-prefixed directories (but not the root),
			// except .buildkite which commonly contains wanda specs.
			if path != searchRoot && len(name) > 0 && (name[0] == '.' || name[0] == '_') {
				if name != ".buildkite" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// Slightly cheaper than suffix on full path: check the entry name first.
		if !strings.HasSuffix(d.Name(), ".wanda.yaml") {
			return nil
		}
		pathsCh <- path
		return nil
	})
	close(pathsCh)

	// 4) Aggregate results (single goroutine => no locking)
	// Note: must drain outCh before checking walkErr to avoid goroutine leak
	for r := range outCh {
		if r.skipped {
			continue
		}

		if existing, ok := index[r.name]; ok && existing != r.path {
			// record conflict paths uniquely
			m := conflicts[r.name]
			if m == nil {
				m = make(map[string]struct{}, 2)
				conflicts[r.name] = m
			}
			m[existing] = struct{}{}
			m[r.path] = struct{}{}

			if minConflictName == "" || r.name < minConflictName {
				minConflictName = r.name
			}
			continue
		}
		index[r.name] = r.path
	}

	// Check walkErr after draining outCh to ensure all goroutines have finished
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", searchRoot, walkErr)
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
