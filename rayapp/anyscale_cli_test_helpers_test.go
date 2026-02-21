package rayapp

import (
	"slices"
	"testing"
)

func checkArgs(
	t *testing.T,
	args, wantCmd, wantFlags []string,
	wantPairs [][2]string,
) {
	t.Helper()
	if len(args) < len(wantCmd) || !slices.Equal(args[:len(wantCmd)], wantCmd) {
		t.Errorf("command = %v, want prefix %v", args, wantCmd)
		return
	}
	opts := args[len(wantCmd):]
	for _, f := range wantFlags {
		if !slices.Contains(opts, f) {
			t.Errorf("args %v missing flag %q", args, f)
		}
	}
	for _, p := range wantPairs {
		if !hasPair(opts, p[0], p[1]) {
			t.Errorf("args %v missing adjacent pair [%q, %q]", args, p[0], p[1])
		}
	}
}

// findPositionalArgs returns the elements of opts that are not part of any
// known flag or flag-value pair. This avoids hardcoding which flags to skip
// when extracting positional arguments.
func findPositionalArgs(opts, flags []string, pairs [][2]string) []string {
	flagSet := make(map[string]struct{}, len(flags))
	for _, f := range flags {
		flagSet[f] = struct{}{}
	}
	pairKeys := make(map[string]struct{}, len(pairs))
	for _, p := range pairs {
		pairKeys[p[0]] = struct{}{}
	}
	var result []string
	for i := 0; i < len(opts); i++ {
		if _, ok := flagSet[opts[i]]; ok {
			continue
		}
		if _, ok := pairKeys[opts[i]]; ok {
			i++ // skip value
			continue
		}
		result = append(result, opts[i])
	}
	return result
}

func hasPair(args []string, key, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}
