package config

import "github.com/sglavoie/tt/internal/util"

const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

// CurrentConfigVersion is the payload schema version shared by the split
// domain files. WorkspaceCatalog.SchemaVersion is the authoritative bundle
// marker; the other payloads deliberately do not duplicate it.
const CurrentConfigVersion = V11ConfigSchemaVersion
