package imgspec

import (
	"bytes"
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

// MakeImage makes a docker image from the spec file.
func MakeImage(specFile string) error {
	bs, err := os.ReadFile(specFile)
	if err != nil {
		return fmt.Errorf("read spec file: %w", err)
	}

	spec := new(Spec)

	dec := yaml.NewDecoder(bytes.NewReader(bs))
	dec.KnownFields(true)

	if err := dec.Decode(spec); err != nil {
		return fmt.Errorf("unmarshal spec file: %w", err)
	}

	return makeImage(spec)
}

func makeImage(spec *Spec) error {
	panic("TODO")
}
