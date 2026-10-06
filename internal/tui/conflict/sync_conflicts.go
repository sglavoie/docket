package conflict

import (
	"fmt"
	"strings"
)

// SyncConflictEntry is one open sync conflict as the browser lists it: the
// file whose versions compete, and each competing version with the devices'
// transactions that left it. The shell converts the sync history's records
// into these, so this package stays a projection that never reads the history.
type SyncConflictEntry struct {
	ID       string
	Path     string
	Kind     string
	Versions []SyncConflictVersion
	// Result is the version the workspace holds meanwhile: the primary one,
	// chosen the same way on every device and never by a timestamp.
	Result  string
	Details []string
}

// SyncConflictVersion is one competing version: its content digest, or empty
// for a deletion, and the transactions that wrote it.
type SyncConflictVersion struct {
	Blob    string
	Writers []string
}

func syncConflictShort(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	return conflictBrowserShortID(id)
}

// syncConflictRowTitle leads with the conflict's short id, like a bundle row,
// and names the file in place of a project.
func syncConflictRowTitle(entry SyncConflictEntry) string {
	return syncConflictShort(entry.ID) + "  " + entry.Path
}

func syncConflictRowSummary(entry SyncConflictEntry) string {
	return fmt.Sprintf("sync conflict · %d versions · %s", len(entry.Versions), entry.Kind)
}

// syncConflictDetailLines is the detail pane for a sync conflict: every
// competing version and who wrote it, the version the workspace holds, and
// the merge details.
func syncConflictDetailLines(entry SyncConflictEntry) []string {
	lines := []string{
		"Sync conflict " + strings.TrimPrefix(entry.ID, "sha256:"),
		"  File:      " + entry.Path,
		"  Kind:      " + entry.Kind,
		"  Holding:   " + syncConflictBlob(entry.Result),
		"",
		fmt.Sprintf("%d competing versions:", len(entry.Versions)),
	}
	for i, version := range entry.Versions {
		lines = append(lines, fmt.Sprintf("  [%d] %s", i+1, syncConflictBlob(version.Blob)))
		for _, writer := range version.Writers {
			lines = append(lines, "      written by "+syncConflictShort(writer))
		}
	}
	if len(entry.Details) > 0 {
		lines = append(lines, "", "Details:")
		for _, detail := range entry.Details {
			lines = append(lines, "  "+detail)
		}
	}
	return append(lines, "", "Every version is kept in the sync history; nothing was lost.")
}

func syncConflictBlob(digest string) string {
	if digest == "" {
		return "deleted"
	}
	return "content " + syncConflictShort(digest)
}
