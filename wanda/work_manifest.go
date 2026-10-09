package wanda

import (
	"fmt"

	cranename "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const buildIDAnnotation = "io.ray.rayci.build-id"

// putWorkManifest tags a copy of the cached manifest stamped with the build ID,
// so each build gets its own manifest and none hits the registry's per-manifest tag cap.
func putWorkManifest(
	workTag cranename.Tag, cache *remote.Descriptor, buildID string, opts ...remote.Option,
) error {
	// Multi-platform images are tagged as before; copying would keep only one platform.
	if cache.MediaType.IsIndex() {
		return remote.Tag(workTag, cache, opts...)
	}
	img, err := cache.Image()
	if err != nil {
		return fmt.Errorf("read cache image: %w", err)
	}
	annotated := mutate.Annotations(img, map[string]string{buildIDAnnotation: buildID})
	if err := remote.Put(workTag, annotated, opts...); err != nil {
		return fmt.Errorf("put work manifest: %w", err)
	}
	return nil
}
