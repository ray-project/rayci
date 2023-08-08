package wanda

// Make makes a container image with the given spec file.
func Make(specFile string, config *ForgeConfig) error {

	return nil
}

// Forge is a forge to build container images.
type Forge struct {
	config *ForgeConfig
}

// NewForge creates a new forge with the given configuration.
func NewForge(config *ForgeConfig) *Forge {
	return &Forge{
		config: config,
	}
}

// ForgeConfig is a configuration for a forge to build container images.
type ForgeConfig struct {
	CacheRepository string
}

// Spec is a specification for a container image.
type Spec struct {
	Name string
	Tags []string

	ContextFiles []string
	BuildFiles   []string

	BuildArgs  map[string]string
	BaseImages []string
}

func MakeSpec(spec *Spec) error {
	return nil
}
