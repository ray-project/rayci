package wanda

import (
	"fmt"
	"log"

	cranename "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// checkRemoteCache checks if the image exists in the remote cache.
// If the cache hits, it tags the work tag with the cached image.
func (f *Forge) checkRemoteCache(cacheTag, workTag string) (bool, error) {
	ct, err := cranename.NewTag(cacheTag)
	if err != nil {
		return false, fmt.Errorf("parse cache tag %q: %w", cacheTag, err)
	}
	wt, err := cranename.NewTag(workTag)
	if err != nil {
		return false, fmt.Errorf("parse work tag %q: %w", workTag, err)
	}

	desc, err := remote.Get(ct, f.remoteOpts...)
	if err != nil {
		log.Printf("cache image miss: %v", err)
		return false, nil
	}

	log.Printf("cache hit: %s", desc.Digest)
	f.cacheHitCount++

	log.Printf("tag output as %s", workTag)
	if err := remote.Tag(wt, desc, f.remoteOpts...); err != nil {
		return false, fmt.Errorf("tag cache image: %w", err)
	}

	return true, nil
}

// checkLocalCache checks if the image exists in the local cache.
// If the cache hits, it tags all output tags with the cached image.
func (f *Forge) checkLocalCache(cacheTag string, tags []string) (bool, error) {
	info, err := f.docker.inspectImage(cacheTag)
	if err != nil {
		return false, fmt.Errorf("check cache image: %w", err)
	}
	if info == nil {
		return false, nil
	}

	log.Printf("cache hit: %s", info.ID)
	f.cacheHitCount++

	for _, tag := range tags {
		log.Printf("tag output as %s", tag)
		if tag != cacheTag {
			if err := f.docker.tag(cacheTag, tag); err != nil {
				return false, fmt.Errorf("tag cache image: %w", err)
			}
		}
	}

	return true, nil
}
