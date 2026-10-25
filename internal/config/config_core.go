package config

import "github.com/sglavoie/docket/internal/util"

const (
	DirPermissions  = util.DirPermissions
	FilePermissions = util.FilePermissions
)

// CurrentConfigVersion is the payload schema version shared by the split
// domain files. WorkspaceCatalog.SchemaVersion is the authoritative bundle
// marker; the other payloads deliberately do not duplicate it.
const CurrentConfigVersion = V23ConfigSchemaVersion

// SyncEnrolledConfigVersion is the catalog schema enrollment and joining write
// into the working copy they create. From schema 23 it is the current schema:
// the catalog's sync block alone marks enrollment, and every tt older than
// this one already refuses schema 23 as newer than it supports, so a number
// reserved for enrolled catalogs would buy nothing. Before schema 23 enrollment
// wrote V22ConfigSchemaVersion, which only an enrolled catalog may still carry
// and which the schema-23 boundary migrates like schema 21.
const SyncEnrolledConfigVersion = CurrentConfigVersion

// SupportedConfigVersion is the newest catalog schema this build reads. A
// catalog beyond it is refused as newer than supported, before anything is
// recovered or written.
const SupportedConfigVersion = CurrentConfigVersion
