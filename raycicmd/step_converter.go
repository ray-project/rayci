package raycicmd

import (
	"fmt"
)

type stepConverter interface {
	// match checks if a step can be converted by the converter.
	match(step map[string]any) bool

	// convert converts a step from the rayci format to the buildkite format.
	convert(id string, step map[string]any) (map[string]any, error)
}

type basicStepConverter struct {
	signatureKey string

	allowedKeys []string
	dropKeys    []string
}

func stepHasKey(step map[string]any, k string) bool {
	_, ok := step[k]
	return ok
}

func (c *basicStepConverter) match(step map[string]any) bool {
	return stepHasKey(step, c.signatureKey)
}

func (c *basicStepConverter) convert(id string, step map[string]any) (
	map[string]any, error,
) {
	if err := checkStepKeys(step, c.allowedKeys); err != nil {
		return nil, fmt.Errorf("check %s step keys: %w", c.signatureKey, err)
	}
	return cloneMapExcept(step, c.dropKeys), nil
}

var triggerConverter = &basicStepConverter{
	signatureKey: "trigger",
	allowedKeys:  triggerStepAllowedKeys,
	dropKeys:     triggerStepDropKeys,
}

var waitConverter = &basicStepConverter{
	signatureKey: "wait",
	allowedKeys:  waitStepAllowedKeys,
	dropKeys:     waitStepDropKeys,
}

var blockConverter = &basicStepConverter{
	signatureKey: "block",
	allowedKeys:  blockStepAllowedKeys,
	dropKeys:     blockStepDropKeys,
}

func isBlockOrWait(step map[string]any) bool {
	return stepHasKey(step, "wait") || stepHasKey(step, "block")
}
