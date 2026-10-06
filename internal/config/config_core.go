package config

import "github.com/sglavoie/tt/internal/util"

const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

// CurrentConfigVersion is the payload schema version shared by the split
// domain files. WorkspaceCatalog.SchemaVersion is the authoritative bundle
// marker; the other payloads deliberately do not duplicate it.
const CurrentConfigVersion = V21ConfigSchemaVersion

// SyncEnrolledConfigVersion is the catalog schema of a workspace enrolled in
// device sync. It is not a step in the migration chain: nothing migrates a
// workspace to it, and a workspace that is not enrolled stays at
// CurrentConfigVersion. Enrollment writes it, together with the catalog's sync
// block, into the working copy it creates, and only there, so a tt that
// predates sync refuses an enrolled working copy as newer than it supports
// while it keeps opening every workspace that was never enrolled.
const SyncEnrolledConfigVersion = 22

// SupportedConfigVersion is the newest catalog schema this build reads. A
// catalog beyond it is refused as newer than supported, before anything is
// recovered or written.
const SupportedConfigVersion = SyncEnrolledConfigVersion
