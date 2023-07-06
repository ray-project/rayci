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

type Envs interface {
	Lookup(string) (string, bool)
}

type osEnvs struct{}

func (e *osEnvs) Lookup(name string) (string, bool) {
	return os.LookupEnv(name)
}
