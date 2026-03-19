package raycicmd

import "testing"

func TestIsTrustedCacheUser(t *testing.T) {
	tests := []struct {
		name  string
		teams string
		want  bool
	}{
		{name: "in trusted team", teams: "builders:trusted:viewers", want: true},
		{name: "only trusted", teams: "trusted", want: true},
		{name: "not in trusted team", teams: "builders:viewers", want: false},
		{name: "env var not set", teams: "", want: false},
		{name: "partial match", teams: "trusted-other:viewers", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := map[string]string{}
			if tt.teams != "" {
				m["BUILDKITE_BUILD_CREATOR_TEAMS"] = tt.teams
			}
			envs := newEnvsMap(m)
			if got := isTrustedCacheUser(envs); got != tt.want {
				t.Errorf(
					"isTrustedCacheUser() = %v, want %v",
					got, tt.want,
				)
			}
		})
	}
}
