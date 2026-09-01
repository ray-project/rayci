package wanda

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	cranename "github.com/google/go-containerregistry/pkg/name"
	crane "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// buildSession holds the shared context for a build or digest run.
type buildSession struct {
	forge *Forge
	graph *depGraph
}

// newBuildSession sets up a Forge and builds the dependency graph for specFile.
// It normalizes config and resolves the env lookup chain.
func newBuildSession(specFile string, config *ForgeConfig) (*buildSession, error) {
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
			return nil, fmt.Errorf("parse envfile: %w", err)
		}
		lookup = func(key string) (string, bool) {
			if v, ok := envfileVars[key]; ok {
				return v, true
			}
			return os.LookupEnv(key)
		}
	}

	s := new(buildSession)

	var err error
	s.graph, err = buildDepGraph(specFile, lookup, config.NamePrefix, wandaSpecsFile)
	if err != nil {
		return nil, fmt.Errorf("build dep graph: %w", err)
	}

	s.forge, err = NewForge(config)
	if err != nil {
		return nil, fmt.Errorf("make forge: %w", err)
	}
	s.forge.lookup = lookup

	return s, nil
}

// Build builds a container image from the given specification file, and builds
// all its dependencies in topological order.
// In RayCI mode, dependencies are assumed built by prior pipeline steps; only
// the root is built.
func Build(specFile string, config *ForgeConfig) error {
	s, err := newBuildSession(specFile, config)
	if err != nil {
		return err
	}

	// In RayCI mode, only build the root (deps built by prior pipeline steps).
	order := s.graph.Order
	if config.RayCI {
		order = []string{s.graph.Root}
	}

	var targetCacheHit bool
	for _, name := range order {
		rs := s.graph.Specs[name]

		log.Printf("building %s (from %s)", name, rs.Path)

		hitsBefore := s.forge.cacheHit()
		if err := s.forge.Build(rs.Spec); err != nil {
			return fmt.Errorf("build %s: %w", name, err)
		}
		if name == s.graph.Root {
			targetCacheHit = s.forge.cacheHit() > hitsBefore
		}
	}

	// Extract artifacts only for the root spec, and only if it was actually
	// built (not a cache hit). On cache hit, the image exists only in the
	// remote registry; extracting would require pulling it first.
	if config.ArtifactsDir != "" {
		rootSpec := s.graph.Specs[s.graph.Root].Spec
		if len(rootSpec.Artifacts) > 0 && !targetCacheHit {
			rootTag := s.forge.workTag(rootSpec.Name)
			if err := s.forge.ExtractArtifacts(rootSpec, rootTag); err != nil {
				return fmt.Errorf("extract artifacts: %w", err)
			}
		} else if targetCacheHit && len(rootSpec.Artifacts) > 0 {
			log.Printf("skipping artifact extraction: cache hit")
		}
	}

	return nil
}

// Forge is a forge to build container images.
type Forge struct {
	config *ForgeConfig

	workDir string

	remoteOpts []remote.Option

	cacheHitCount int

	docker *dockerCmd

	lookup lookupFunc
}

// NewForge creates a new forge with the given configuration.
func NewForge(config *ForgeConfig) (*Forge, error) {
	if err := checkPlatformSupport(); err != nil {
		return nil, err
	}

	absWorkDir, err := filepath.Abs(filepath.FromSlash(config.WorkDir))
	if err != nil {
		return nil, fmt.Errorf("abs path for work dir: %w", err)
	}

	f := &Forge{
		config:  config,
		workDir: absWorkDir,
		remoteOpts: []remote.Option{
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
			remote.WithPlatform(crane.Platform{
				OS:           targetOS(),
				Architecture: runtime.GOARCH,
			}),
		},
	}
	f.docker = f.newDockerCmd()

	return f, nil
}

func (f *Forge) cacheHit() int { return f.cacheHitCount }

func (f *Forge) addSrcFile(ts *tarStream, src string) {
	ts.addFile(src, nil, filepath.Join(f.workDir, filepath.FromSlash(src)))
}

func (f *Forge) isRemote() bool             { return f.config.isRemote() }
func (f *Forge) workTag(name string) string { return f.config.workTag(name) }
func (f *Forge) cacheTag(digest string) string {
	return f.config.cacheTag(digest)
}

func (f *Forge) newDockerCmd() *dockerCmd {
	return newDockerCmd(&dockerCmdConfig{
		bin:             f.config.DockerBin,
		useLegacyEngine: runtime.GOOS == "windows",
	})
}

// isDockerScratch reports whether s is Docker's built-in empty base image.
// "scratch" is not a real registry image; Docker handles it as a special
// keyword in FROM instructions, so it must not be pulled or resolved.
func isDockerScratch(s string) bool {
	return s == "scratch"
}

func (f *Forge) resolveBases(froms []string) (map[string]*imageSource, error) {
	m := make(map[string]*imageSource)
	namePrefix := f.config.NamePrefix

	for _, from := range froms {
		if isDockerScratch(from) {
			m[from] = &imageSource{
				name:  from,
				id:    from,
				local: from,
			}
			continue
		}

		if strings.HasPrefix(from, "@") { // A local image.
			name := strings.TrimPrefix(from, "@")
			src, err := resolveDockerImage(f.docker, from, name)
			if err != nil {
				return nil, fmt.Errorf("resolve local image %s: %w", from, err)
			}
			m[from] = src
			continue
		}

		if namePrefix != "" && strings.HasPrefix(from, namePrefix) {
			if !f.isRemote() {
				// Treat it as a local image.
				src, err := resolveDockerImage(f.docker, from, from)
				if err != nil {
					return nil, fmt.Errorf(
						"resolve prefixed local image %s: %w", from, err,
					)
				}
				m[from] = src
				continue
			}

			// An image in the work namespace. It is generated by a previous
			// job, and we need to pull it from the work repo.
			fromName := strings.TrimPrefix(from, f.config.NamePrefix)
			workTag := f.workTag(fromName)

			src, err := resolveRemoteImage(from, workTag, f.remoteOpts...)
			if err != nil {
				return nil, fmt.Errorf(
					"resolve remote work image %s: %w", from, err,
				)
			}
			m[from] = src
			continue
		}

		// A normal remote image that we need to pull from the network.
		// Try local image first, fallback to remote if not found locally.
localSrc, localErr := resolveDockerImage(f.docker, from, from)
		if localErr == nil {
			log.Printf("using local image for %s", from)
			m[from] = localSrc
			continue
		}

		// Local image not found, try remote
		src, err := resolveRemoteImage(from, from, f.remoteOpts...)
		if err != nil {
return nil, fmt.Errorf("resolve remote image %s: %w", from, err)
		}
		m[from] = src
	}
	return m, nil
}

// ExtractArtifacts copies artifacts from a built image to ArtifactsDir.
// The image must be locally available (this is only called after a successful
// build, not on cache hits).
func (f *Forge) ExtractArtifacts(spec *Spec, imageTag string) error {
	d := f.newDockerCmd()
	artifactsDir := f.config.ArtifactsDir

	// In RayCI mode, clear the artifacts directory to avoid stale artifacts.
	if f.config.RayCI {
		if err := os.RemoveAll(artifactsDir); err != nil {
			return fmt.Errorf("clear artifacts dir: %w", err)
		}
	}

	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return fmt.Errorf("create artifacts dir: %w", err)
	}

	log.Printf("extracting %d artifact(s) from %s", len(spec.Artifacts), imageTag)
	extractStart := time.Now()

	containerID, err := d.createContainer(imageTag)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	defer func() {
		if err := d.removeContainer(containerID); err != nil {
			log.Printf("warning: failed to remove container %s: %v", containerID, err)
		}
	}()

	var extracted []string

	for _, a := range spec.Artifacts {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("invalid artifact: %w", err)
		}

		dst, err := a.ResolveDst(artifactsDir)
		if err != nil {
			return fmt.Errorf("resolve artifact dst: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("create dir for artifact %s: %w", a.Dst, err)
		}

		if err := d.copyFromContainer(containerID, a.Src, dst); err != nil {
			if a.Optional {
				log.Printf("warning: optional artifact not found: %s", a.Src)
				continue
			}
			return fmt.Errorf("copy artifact %s: %w", a.Src, err)
		}
		if abs, err := filepath.Abs(dst); err == nil {
			dst = abs
		}
		extracted = append(extracted, dst)
	}

	log.Printf("extracted %d artifact(s) in %v:", len(extracted), time.Since(extractStart).Round(time.Millisecond))
	for _, f := range extracted {
		log.Printf("  %s", f)
	}
	return nil
}

// resolveBuildInput assembles the build input and core for a spec.
// This is the shared setup used by both Build and digestSpec.
func (f *Forge) resolveBuildInput(spec *Spec) (*buildInput, *buildInputCore, error) {
	ts := newTarStream()

	if spec.ContextOwner != "" {
		owner, err := parseContextOwner(spec.ContextOwner)
		if err != nil {
			return nil, nil, err
		}
		ts.owner = owner
	}

	files, err := listSrcFiles(f.workDir, spec.Srcs, spec.Dockerfile)
	if err != nil {
		return nil, nil, fmt.Errorf("list src files: %w", err)
	}
	for _, file := range files {
		f.addSrcFile(ts, file)
	}

	in := newBuildInput(ts, spec.BuildArgs)

	froms, err := f.resolveBases(spec.Froms)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve bases: %w", err)
	}
	in.froms = froms

	inputCore, err := in.makeCore(spec.Dockerfile, f.lookup)
	if err != nil {
		return nil, nil, fmt.Errorf("make build input core: %w", err)
	}
	inputCore.Epoch = f.config.Epoch
	inputCore.ContextOwner = spec.ContextOwner

	return in, inputCore, nil
}

// digestSpec computes the content-addressed digest for the given spec without
// checking the cache or building the image.
func (f *Forge) digestSpec(spec *Spec) (string, error) {
	_, inputCore, err := f.resolveBuildInput(spec)
	if err != nil {
		return "", err
	}
	return inputCore.digest()
}

// Digest computes and writes the content-addressed digest for the given spec
// file to w. It performs all the standard digest-generation steps (resolving
// base images, hashing the build context, expanding build args) but does not
// check the cache or build the image.
func Digest(specFile string, config *ForgeConfig, w io.Writer) error {
	s, err := newBuildSession(specFile, config)
	if err != nil {
		return err
	}

	inputDigest, err := s.forge.digestSpec(s.graph.Specs[s.graph.Root].Spec)
	if err != nil {
		return fmt.Errorf("compute digest for %s: %w", s.graph.Root, err)
	}

	fmt.Fprintln(w, inputDigest)
	return nil
}

// Build builds a container image from the given specification.
func (f *Forge) Build(spec *Spec) error {
	in, inputCore, err := f.resolveBuildInput(spec)
	if err != nil {
		return err
	}

	caching := !spec.DisableCaching

	inputDigest, err := inputCore.digest()
	if err != nil {
		return fmt.Errorf("compute build input digest: %w", err)
	}
	log.Println("build input digest:", inputDigest)

	cacheTag := f.cacheTag(inputDigest)
	workTag := f.workTag(spec.Name)

	// Add all the tags.

	// Work tag is the tag we use to save the image in the work repo.
	in.addTag(workTag)
	in.addTag(cacheTag)

	// When running on rayCI, we only need the workTag and the cacheTag.
	// Otherwise, add extra tags.
	if !f.config.RayCI {
		// Name tag is the tag we use to reference the image locally.
		// It is also what can be referenced by following steps.
		if f.config.NamePrefix != "" {
			nameTag := f.config.NamePrefix + spec.Name
			in.addTag(nameTag)
		}
		for _, tag := range spec.Tags { // And add extra tags.
			in.addTag(tag)
		}
	}

	if caching && !f.config.Rebuild {
		if f.isRemote() {
			ct, err := cranename.NewTag(cacheTag)
			if err != nil {
				return fmt.Errorf("parse cache tag %q: %w", cacheTag, err)
			}
			wt, err := cranename.NewTag(workTag)
			if err != nil {
				return fmt.Errorf("parse work tag %q: %w", workTag, err)
			}

			desc, err := remote.Get(ct, f.remoteOpts...)
			if err != nil {
				log.Printf("cache image miss: %v", err)
			} else {
				log.Printf("cache hit: %s", desc.Digest)
				f.cacheHitCount++

				log.Printf("tag output as %s", workTag)
				if err := remote.Tag(wt, desc, f.remoteOpts...); err != nil {
					return fmt.Errorf("tag cache image: %w", err)
				}

				return nil // and we are done.
			}
		} else {
			info, err := f.docker.inspectImage(cacheTag)
			if err != nil {
				return fmt.Errorf("check cache image: %w", err)
			}
			if info != nil {
				log.Printf("cache hit: %s", info.ID)
				f.cacheHitCount++

				for _, tag := range in.tagList() {
					log.Printf("tag output as %s", tag)
					if tag != cacheTag {
						if err := f.docker.tag(cacheTag, tag); err != nil {
							return fmt.Errorf("tag cache image: %w", err)
						}
					}
				}
				return nil // and we are done.
			}
		}
	}

	inputHints := newBuildInputHints(spec.BuildHintArgs, f.lookup)

	// Now we can build the image.
	// Always use a new dockerCmd so that it can run in its own environment.
	d := f.newDockerCmd()
	d.setWorkDir(f.workDir)

	if err := d.build(in, inputCore, inputHints); err != nil {
		return fmt.Errorf("build docker: %w", err)
	}

	// Push the image to the work repo with workTag and cacheTag if needed.
	if f.isRemote() {
		if err := d.run("push", workTag); err != nil {
			return fmt.Errorf("push docker: %w", err)
		}

		// Save cache result too.
		if caching && !f.config.ReadOnlyCache {
			if err := d.run("push", cacheTag); err != nil {
				return fmt.Errorf("push cache: %w", err)
			}
		}
	}

	return nil
}
