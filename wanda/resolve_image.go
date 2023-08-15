package wanda

import (
	"fmt"

	cranename "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func resolveRemoteImage(name, ref string, opts ...remote.Option) (
	*imageSource, error,
) {
	parsed, err := cranename.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("parse reference %s: %w", ref, err)
	}
	img, err := remote.Image(parsed, opts...)
	if err != nil {
		return nil, fmt.Errorf("fetch image %s: %w", ref, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("get digest for %s: %w", ref, err)
	}

	id, err := img.ConfigName()
	if err != nil {
		return nil, fmt.Errorf("get config name/id for %s: %w", ref, err)
	}

	src := parsed.Context().Digest(digest.String())

	return &imageSource{
		name: name,
		id:   id.String(),
		src:  src.String(),
	}, nil
}

func resolveLocalImage(name, ref string) (*imageSource, error) {
	parsed, err := cranename.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("parse reference %s: %w", ref, err)
	}

	img, err := daemon.Image(parsed)
	if err != nil {
		return nil, fmt.Errorf("fetch image %s: %w", ref, err)
	}
	id, err := img.ConfigName()
	if err != nil {
		return nil, fmt.Errorf("get config name/id for %s: %w", ref, err)
	}

	return &imageSource{
		name: name,
		id:   id.String(),
	}, nil
}
