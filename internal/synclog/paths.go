package synclog

import (
	"fmt"
	"strings"
)

// localInternalDirs are the .tt/ subtrees that are device-local by definition:
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
	case ".tt":
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
