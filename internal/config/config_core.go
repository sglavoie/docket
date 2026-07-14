package config

import "github.com/sglavoie/tt/internal/util"

const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

const DefaultUndoHistoryDepth = 50

// CurrentConfigVersion is the schema version shared by the split domain files.
const CurrentConfigVersion = 3
