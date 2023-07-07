package raycicmd

import (
	"os"
)

func getEnv(envs Envs, k string) string {
	if v, ok := envs.Lookup(k); ok {
		return v
	}
	return ""
}

// Envs is an interface for looking up environment variables.
type Envs interface {
	// Lookup returns the value of the environment variable named by the key.
	// If the variable is not present, it returns false.
	Lookup(string) (string, bool)
}

type osEnvs struct{}

func (e *osEnvs) Lookup(name string) (string, bool) {
	return os.LookupEnv(name)
}

// envsMap is a map of environment variables for stubbing and testing.
type envsMap struct {
	m map[string]string
}

func newEnvsMap(m map[string]string) *envsMap {
	return &envsMap{m: m}
}

func (m *envsMap) Lookup(name string) (string, bool) {
	v, ok := m.m[name]
	return v, ok
}
