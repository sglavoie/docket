package synclog

import (
	"fmt"
	"strings"
)

const (
	// internalDir is the workspace's internal directory.
	internalDir = ".docket"
	// legacyInternalDir is the internal directory before schema 24. Records a
	// device wrote before the move still name it; CanonicalPath reads those
	// paths as .docket/ ones, and the tombstone it holds now is never synced.
	legacyInternalDir = ".tt"
)

// CanonicalPath maps a workspace-relative path a pre-v24 record wrote under
// .tt/ onto the .docket/ path the same file has now, and returns every other
// path unchanged. Immutable records keep the bytes they were sealed with, so the
// translation happens wherever a recorded path is read.
func CanonicalPath(rel string) string {
	if strings.HasPrefix(rel, legacyInternalDir+"/") {
		return internalDir + strings.TrimPrefix(rel, legacyInternalDir)
	}
	return rel
}

// localInternalDirs are the .docket/ subtrees that are device-local by definition:
// the journal, the undo ledger, this history, snapshots, and conflict bundles.
var localInternalDirs = map[string]bool{
	"transactions": true,
	"undo":         true,
	"sync":         true,
	"backups":      true,
	"conflicts":    true,
}

// Classify maps a workspace-relative, '/'-separated path to the effect kind a
// change to it publishes. local reports a path that is never exchanged —
// journal and ledger state, this history, backups, conflict bundles, lock files,
// staging files, and .bak sidecars. slug is the owning project's directory name
// for a project-scoped kind. Any other path is ErrUnclassifiedPath: a commit
// cannot publish a transaction whose effects it cannot name.
func Classify(rel string) (kind EffectKind, slug string, local bool, err error) {
	parts := strings.Split(rel, "/")
	base := parts[len(parts)-1]
	if strings.HasSuffix(base, ".bak") || strings.HasSuffix(base, ".lock") || strings.HasPrefix(base, ".tt-stage-") {
		return "", "", true, nil
	}
	switch parts[0] {
	case legacyInternalDir:
		// The tombstone each device keeps where .tt/ was; a record naming a
		// pre-v24 path is classified through CanonicalPath instead.
		return "", "", true, nil
	case internalDir:
		if len(parts) == 2 {
			switch parts[1] {
			case "workspace.json":
				return EffectCatalog, "", false, nil
			case "planning.json":
				return EffectPlanning, "", false, nil
			case "lenses.json":
				return EffectLenses, "", false, nil
			}
		}
		if len(parts) >= 2 && localInternalDirs[parts[1]] {
			return "", "", true, nil
		}
		if len(parts) >= 3 && parts[1] == "global-notes" {
			return EffectGlobalNote, "", false, nil
		}
	case "projects":
		if len(parts) >= 3 && parts[1] != "" {
			slug = parts[1]
			switch {
			case len(parts) == 3 && parts[2] == "tasks.md":
				return EffectProjectDocument, slug, false, nil
			case parts[2] == "notes":
				return EffectNote, slug, false, nil
			case parts[2] == ".attachments":
				return EffectAttachment, slug, false, nil
			default:
				return EffectProjectTree, slug, false, nil
			}
		}
	}
	return "", "", false, fmt.Errorf("%w: %s", ErrUnclassifiedPath, rel)
}
