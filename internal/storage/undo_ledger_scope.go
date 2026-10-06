package storage

import (
	"path/filepath"
	"strings"
)

// LedgerScope selects which history line an undo/redo targets: the whole
// workspace, the newest entry affecting one project, the newest entry touching
// an artifact root, or the intersection of project and artifact restrictions.
type LedgerScope struct {
	// ProjectID, when non-empty, restricts selection to the newest entry that
	// touched that project (current-project undo).
	ProjectID string
	// ArtifactRoot, when non-empty, restricts selection to entries with at least
	// one artifact lexically contained by this path. Matching deliberately does
	// not resolve symlinks or require the artifact to exist: undo entries commonly
	// describe files that a create, delete, rename, or move made absent.
	ArtifactRoot string
}

// GlobalScope targets the newest entry across the whole workspace.
func GlobalScope() LedgerScope { return LedgerScope{} }

// SelectNewestUndo returns the entry a subsequent Undo(scope) would reverse
// without applying it, and whether one exists. It is the read-only counterpart
// callers use to preview or gate the availability of undo.
func (l *WorkspaceLedger) SelectNewestUndo(scope LedgerScope) (LedgerEntry, bool, error) {
	lf, err := l.load()
	if err != nil {
		return LedgerEntry{}, false, err
	}
	entry, _, ok := selectNewestOwned(lf.Entries, scope, l.device())
	return entry, ok, nil
}

// UndoCount reports how many undo entries the scope can still reach; the CLI
// uses it to tell the user how many steps remain after a restore.
func (l *WorkspaceLedger) UndoCount(scope LedgerScope) (int, error) {
	lf, err := l.load()
	if err != nil {
		return 0, err
	}
	return countInScope(lf.Entries, scope, l.device()), nil
}

// RedoCount reports how many redo entries the scope can still reach.
func (l *WorkspaceLedger) RedoCount(scope LedgerScope) (int, error) {
	lf, err := l.load()
	if err != nil {
		return 0, err
	}
	return countInScope(lf.RedoEntries, scope, l.device()), nil
}

func countInScope(stack []LedgerEntry, scope LedgerScope, device string) int {
	n := 0
	for i := range stack {
		if entryMatchesScope(stack[i], scope) && stack[i].ownedBy(device) {
			n++
		}
	}
	return n
}

// entryMatchesScope is the single matching rule shared by selection and stack
// counts. Non-empty restrictions combine with AND semantics; the zero-value
// scope therefore matches every workspace entry.
func entryMatchesScope(entry LedgerEntry, scope LedgerScope) bool {
	if scope.ProjectID != "" && !entry.touchesProject(scope.ProjectID) {
		return false
	}
	if scope.ArtifactRoot == "" {
		return true
	}
	for _, artifact := range entry.Artifacts {
		if pathWithinArtifactRoot(scope.ArtifactRoot, artifact.Path) {
			return true
		}
		// A directory image also touches scopes nested below that directory. This
		// matters when callers scope history to one attachment file or subtree.
		if artifact.imageKind() == ArtifactImageKindDirectory && pathWithinArtifactRoot(artifact.Path, scope.ArtifactRoot) {
			return true
		}
	}
	return false
}

// pathWithinArtifactRoot reports lexical path-component containment only. It
// intentionally avoids filesystem inspection and symlink resolution because a
// restore target may currently be absent.
func pathWithinArtifactRoot(root, target string) bool {
	if root == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// SelectNewestRedo returns the entry a subsequent Redo(scope) would replay.
func (l *WorkspaceLedger) SelectNewestRedo(scope LedgerScope) (LedgerEntry, bool, error) {
	lf, err := l.load()
	if err != nil {
		return LedgerEntry{}, false, err
	}
	entry, _, ok := selectNewestOwned(lf.RedoEntries, scope, l.device())
	return entry, ok, nil
}
