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

type waitConverter struct{}

func newWaitConverter() *waitConverter { return &waitConverter{} }

func (c *waitConverter) match(step map[string]any) bool {
	_, ok := step["wait"]
	return ok
}

func (c *waitConverter) convert(step map[string]any) (map[string]any, error) {
	// a wait step
	if err := checkStepKeys(step, waitStepAllowedKeys); err != nil {
		return nil, fmt.Errorf("check wait step keys: %w", err)
	}
	return cloneMapExcept(step, waitStepDropKeys), nil
}

type blockConverter struct{}

func newBlockConverter() *blockConverter { return &blockConverter{} }

func (c *blockConverter) match(step map[string]any) bool {
	_, ok := step["block"]
	return ok
}

func (c *blockConverter) convert(step map[string]any) (map[string]any, error) {
	// a block step
	if err := checkStepKeys(step, blockStepAllowedKeys); err != nil {
		return nil, fmt.Errorf("check block step keys: %w", err)
	}
	return cloneMapExcept(step, blockStepDropKeys), nil
}
