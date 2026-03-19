package raycicmd

import "strings"

const trustedCacheTeam = "trusted"

func isTrustedCacheUser(envs Envs) bool {
	teams := getEnv(envs, "BUILDKITE_BUILD_CREATOR_TEAMS")
	if teams == "" {
		return false
	}
	for _, team := range strings.Split(teams, ":") {
		if team == trustedCacheTeam {
			return true
		}
	}
	return false
}
