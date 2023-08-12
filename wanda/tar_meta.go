package wanda

import (
	"os"
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
