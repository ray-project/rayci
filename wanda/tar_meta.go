package wanda

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type tarMeta struct {
	Mode    int64
	UserID  int
	GroupID int
}

func tarMetaFromFileInfo(info os.FileInfo) *tarMeta {
	const rootUser = 0
	return &tarMeta{
		Mode:    int64(info.Mode()) & 0777,
		UserID:  rootUser,
		GroupID: rootUser,
	}
}

// contextOwner holds a uid:gid pair parsed from a wanda spec.
type contextOwner struct {
	UserID  int
	GroupID int
}

// parseContextOwner parses a "uid:gid" string.
func parseContextOwner(s string) (*contextOwner, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf(
			"context_owner %q: expected uid:gid format",
			s,
		)
	}
	uid, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf(
			"context_owner uid %q: %w", parts[0], err,
		)
	}
	if uid < 0 {
		return nil, fmt.Errorf(
			"context_owner uid %q: must be non-negative", parts[0],
		)
	}
	gid, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf(
			"context_owner gid %q: %w", parts[1], err,
		)
	}
	if gid < 0 {
		return nil, fmt.Errorf(
			"context_owner gid %q: must be non-negative", parts[1],
		)
	}
	return &contextOwner{UserID: uid, GroupID: gid}, nil
}
