package raycicmd

import (
	"testing"

	"os"
)

func TestOSEnv(t *testing.T) {
	envs := &osEnvs{}
	home := os.Getenv("HOME")
	if got := getEnv(envs, "HOME"); got != home {
		t.Errorf("get HOME: got %q, want %q", got, home)
	}
}

func TestEnvsMap(t *testing.T) {
	const fakeHome = "/opt/fakehome"
	m := newEnvsMap(map[string]string{"HOME": fakeHome})
	if got := getEnv(m, "HOME"); got != fakeHome {
		t.Errorf("get HOME: got %q, want %q", got, fakeHome)
	}

	if v, ok := m.Lookup("PATH"); ok {
		t.Errorf("got PATH %q, want not exist", v)
	}
}
