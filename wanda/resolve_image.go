package wanda

import (
	"fmt"

	cranename "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
)

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
