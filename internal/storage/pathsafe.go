package storage

import "github.com/sglavoie/docket/internal/util"

// This file is the single storage-level entry point for filesystem containment.
// Every create/rename/delete that combines a workspace root directory with an
// externally-influenced segment (project slug, task ID, note or attachment
// filename) must resolve the final path through SafeChildName or SafeJoin so a
// crafted value cannot escape the workspace via absolute paths, "." / ".."
// components, path separators, or symlink traversal.

// SafeChildName validates that name is a safe single path component (a bare
// filename, slug, or task ID) that resolves to a direct child of a directory.
// It rejects empty names, absolute paths, dot components, and both Unix and
// Windows separators. See util.ValidateChildName for the sentinel errors.
func SafeChildName(name string) error {
	return util.ValidateChildName(name)
}

// SafeJoin joins rel beneath root and guarantees the result stays inside root,
// rejecting absolute references, "." / ".." components, and symlink traversal
// that escapes root. It returns the cleaned, contained path. Use it before any
// storage-level rename or recursive delete whose target is derived from a slug
// or other externally-influenced identifier.
func SafeJoin(root, rel string) (string, error) {
	return util.SafeJoin(root, rel)
}
