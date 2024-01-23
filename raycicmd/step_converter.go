package raycicmd

import (
	"fmt"
)

type stepConverter interface {
	// match checks if a step can be converted by the converter.
	match(step map[string]any) bool

	// convert converts a step from the rayci format to the buildkite format.
	convert(step map[string]any) (map[string]any, error)
}

type basicConverter struct {
	signatureKey string

	allowedKeys []string
	dropKeys    []string
}

func (c *basicConverter) match(step map[string]any) bool {
	_, ok := step[c.signatureKey]
	return ok
}

func (c *basicConverter) convert(step map[string]any) (map[string]any, error) {
	if err := checkStepKeys(step, c.allowedKeys); err != nil {
		return nil, fmt.Errorf("check wait step keys: %w", err)
	}
	return cloneMapExcept(step, c.dropKeys), nil
}

var waitConverter = &basicConverter{
	signatureKey: "wait",
	allowedKeys:  waitStepAllowedKeys,
	dropKeys:     waitStepDropKeys,
}

var blockConverter = &basicConverter{
	signatureKey: "block",
	allowedKeys:  blockStepAllowedKeys,
	dropKeys:     blockStepDropKeys,
}
