package wanda

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	cranename "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// PredictCacheHit predicts if a wanda build will result in a cache hit
// without actually building the image. It computes the build input digest
// and checks if the corresponding cache tag exists in the registry.
func PredictCacheHit(specFile string, config *ForgeConfig) (bool, error) {
	if config == nil {
		return false, fmt.Errorf("config is required")
	}

	spec, err := parseSpecFile(specFile)
	if err != nil {
		return false, fmt.Errorf("parse spec file: %w", err)
	}

	// Expand env variables just like the actual build does
	spec = spec.expandVar(os.LookupEnv)

	// If caching is disabled, it won't cache hit
	if spec.DisableCaching {
		return false, nil
	}

	// If rebuild is forced, it won't use cache
	if config.Rebuild {
		return false, nil
	}

	// Only predict for remote builds where we can check the registry
	if !config.isRemote() {
		return false, nil
	}

	// Resolve work directory
	absWorkDir, err := filepath.Abs(filepath.FromSlash(config.WorkDir))
	if err != nil {
		return false, fmt.Errorf("abs path for work dir: %w", err)
	}

	// Prepare the tar stream
	ts := newTarStream()
	files, err := listSrcFiles(absWorkDir, spec.Srcs, spec.Dockerfile)
	if err != nil {
		return false, fmt.Errorf("list src files: %w", err)
	}
	for _, file := range files {
		ts.addFile(file, nil, filepath.Join(absWorkDir, filepath.FromSlash(file)))
	}

	// Create build input
	in := newBuildInput(ts, spec.BuildArgs)

	// Resolve base images
	froms, err := resolveBases(spec.Froms, config, absWorkDir)
	if err != nil {
		return false, fmt.Errorf("resolve bases: %w", err)
	}
	in.froms = froms

	// Compute build input core
	inputCore, err := in.makeCore(spec.Dockerfile)
	if err != nil {
		return false, fmt.Errorf("make build input core: %w", err)
	}
	inputCore.Epoch = config.Epoch

	// Compute the digest
	inputDigest, err := inputCore.digest()
	if err != nil {
		return false, fmt.Errorf("compute build input digest: %w", err)
	}

	// Get the cache tag
	cacheTag := config.cacheTag(inputDigest)

	// Check if the cache tag exists in the registry
	ct, err := cranename.NewTag(cacheTag)
	if err != nil {
		return false, fmt.Errorf("parse cache tag %q: %w", cacheTag, err)
	}

	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}

	_, err = remote.Get(ct, remoteOpts...)
	if err != nil {
		// Cache miss or error checking
		return false, nil
	}

	// Cache hit!
	return true, nil
}

// resolveBases resolves base images for cache prediction
func resolveBases(froms []string, config *ForgeConfig, workDir string) (map[string]*imageSource, error) {
	m := make(map[string]*imageSource)
	namePrefix := config.NamePrefix

	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}

	for _, from := range froms {
		// Local images (prefixed with @) - skip for prediction
		// We can't reliably predict local image digests without docker
		if from[0] == '@' {
			log.Printf("skipping local image %s for cache prediction", from)
			return nil, fmt.Errorf("cannot predict cache for local images: %s", from)
		}

		// Work namespace images
		if namePrefix != "" && len(from) > len(namePrefix) && from[:len(namePrefix)] == namePrefix {
			fromName := from[len(namePrefix):]
			workTag := config.workTag(fromName)

			src, err := resolveRemoteImage(from, workTag, remoteOpts...)
			if err != nil {
				return nil, fmt.Errorf("resolve remote work image %s: %w", from, err)
			}
			m[from] = src
			continue
		}

		// Normal remote images
		src, err := resolveRemoteImage(from, from, remoteOpts...)
		if err != nil {
			return nil, fmt.Errorf("resolve remote image %s: %w", from, err)
		}
		m[from] = src
	}

	return m, nil
}
