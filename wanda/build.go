package wanda

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Build builds a container image from the given specification file, and builds
// all its dependencies in topological order.
// In RayCI mode, dependencies are assumed built by prior pipeline steps; only
// the root is built.
func Build(specFile string, config *ForgeConfig) error {
	if config == nil {
		config = &ForgeConfig{}
	}

	wandaSpecsFile := config.WandaSpecsFile
	if wandaSpecsFile == "" {
		wandaSpecsFile = filepath.Join(config.WorkDir, ".wandaspecs")
	}

	lookup := lookupFunc(os.LookupEnv)
	if config.EnvFile != "" {
		envfileVars, err := ParseEnvFile(config.EnvFile)
		if err != nil {
			return fmt.Errorf("parse envfile: %w", err)
		}
		lookup = func(key string) (string, bool) {
			if v, ok := envfileVars[key]; ok {
				return v, true
			}
			return os.LookupEnv(key)
		}
	}

	graph, err := buildDepGraph(specFile, lookup, config.NamePrefix, wandaSpecsFile)
	if err != nil {
		return fmt.Errorf("build dep graph: %w", err)
	}

	forge, err := NewForge(config)
	if err != nil {
		return fmt.Errorf("make forge: %w", err)
	}

	// In RayCI mode, only build the root (deps built by prior pipeline steps).
	order := graph.Order
	if config.RayCI {
		order = []string{graph.Root}
	}

	for _, name := range order {
		rs := graph.Specs[name]

		log.Printf("building %s (from %s)", name, rs.Path)

		if err := forge.Build(rs.Spec); err != nil {
			return fmt.Errorf("build %s: %w", name, err)
		}
	}

	return nil
}
