package storage

import (
	"fmt"
	"path/filepath"
)

// Undo reverses the newest entry in scope, restoring every artifact to its
// before-image, then moves the entry onto the redo stack. It returns the undone
// action description. A global scope selects the newest entry overall; a project
// scope selects the newest entry that touched that project even when later
// entries for other projects sit above it (out-of-order project histories).
func (l *WorkspaceLedger) Undo(scope LedgerScope) (string, error) {
	_, action, err := l.UndoWithEntry(scope)
	return action, err
}

// UndoWithEntry is Undo plus the exact entry selected under the workspace lock.
// Callers that must refresh affected projects use this instead of previewing the
// ledger separately and then racing a concurrent commit before restore begins.
func (l *WorkspaceLedger) UndoWithEntry(scope LedgerScope) (LedgerEntry, string, error) {
	return l.restore(scope, false)
}

// Redo replays the newest entry in scope, restoring every artifact to its
// after-image, then moves the entry back onto the undo stack.
func (l *WorkspaceLedger) Redo(scope LedgerScope) (string, error) {
	_, action, err := l.RedoWithEntry(scope)
	return action, err
}

// RedoWithEntry is Redo plus the exact entry selected under the workspace lock.
func (l *WorkspaceLedger) RedoWithEntry(scope LedgerScope) (LedgerEntry, string, error) {
	return l.restore(scope, true)
}

// restore is the shared undo/redo engine. redo=false consumes the undo stack and
// applies before-images; redo=true consumes the redo stack and applies
// after-images. The consumed entry moves to the opposite stack so the operation
// is itself reversible, matching the linear Vim-style contract.
func (l *WorkspaceLedger) restore(scope LedgerScope, redo bool) (LedgerEntry, string, error) {
	service := NewCommitService(l.transactionsDir, l.roots).OnMachine(l.machineID)
	var restored LedgerEntry
	err := service.withWorkspaceCommitLock(func() error {
		journal := service.journal()
		if _, recoverErr := service.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}

		lf, loadErr := l.load()
		if loadErr != nil {
			return loadErr
		}
		label := "undo"
		entry, idx, ok := selectNewestOwned(lf.Entries, scope, l.device())
		if redo {
			label = "redo"
			entry, idx, ok = selectNextRedoOwned(lf.RedoEntries, scope, l.device())
		}
		if !ok {
			historyErr := ErrNoUndoHistory
			if redo {
				historyErr = ErrNoRedoHistory
			}
			switch {
			case scope.ProjectID != "" && scope.ArtifactRoot != "":
				return fmt.Errorf("%w for project %q under artifact root %q", historyErr, scope.ProjectID, scope.ArtifactRoot)
			case scope.ProjectID != "":
				return fmt.Errorf("%w for project %q", historyErr, scope.ProjectID)
			case scope.ArtifactRoot != "":
				return fmt.Errorf("%w for artifact root %q", historyErr, scope.ArtifactRoot)
			}
			return historyErr
		}

		var directoryLockPaths []string
		for _, image := range entry.Artifacts {
			if image.imageKind() == ArtifactImageKindDirectory {
				directoryLockPaths = append(directoryLockPaths, filepath.Clean(image.Path))
			}
		}
		// Attachment files use the adjacent task-directory lock even when the
		// directory itself did not transition. Creating a per-file lock inside an
		// attachment directory would add a non-user sibling and alter cleanup.
		directoryLockPaths = append(directoryLockPaths, attachmentDirectoryPaths(entry)...)
		directoryLockPaths = outermostDirectoryPaths(directoryLockPaths)
		// Every image the kept directories cover — files and nested directories
		// alike — yields to the one lock above it. A nested lock would sit inside a
		// directory this restore expects to find absent, and
		// createArtifactLockDirectories would recreate it before preflight could
		// read the disk.
		artifacts := make([]ArtifactChange, 0, len(entry.Artifacts)+len(directoryLockPaths))
		for _, image := range entry.Artifacts {
			if pathWithinAnyDirectory(image.Path, directoryLockPaths) {
				continue
			}
			artifacts = append(artifacts, ArtifactChange{Path: image.Path})
		}
		for _, dir := range directoryLockPaths {
			artifacts = append(artifacts, ArtifactChange{Path: dir})
		}
		lockPaths, lockErr := artifactLockPaths(artifacts, l.roots)
		if lockErr != nil {
			return lockErr
		}
		if err := createArtifactLockDirectories(lockPaths); err != nil {
			return err
		}
		return withOrderedFileLocks(lockPaths, func() error {
			manifest, prepareErr := l.prepareRestoreTransaction(journal, lf, entry, idx, redo)
			if prepareErr != nil {
				return l.retireDriftedEntry(journal, lf, entry, idx, redo, prepareErr)
			}
			if commitErr := commitJournal(journal, manifest); commitErr != nil {
				return fmt.Errorf("committing %s transaction: %w", label, commitErr)
			}
			l.gcBlobs(lf)
			restored = entry
			return nil
		})
	})
	if err != nil {
		return LedgerEntry{}, "", err
	}
	return restored, restored.Description(), nil
}

func pathWithinAnyDirectory(path string, dirs []string) bool {
	for _, dir := range dirs {
		if pathWithinArtifactRoot(dir, path) {
			return true
		}
	}
	return false
}

// outermostDirectoryPaths drops any directory contained in another one from the
// same entry, leaving exactly the set of locks that covers every image without
// placing one inside another. Whole-tree deletes — a project with its notes and
// attachments — are what make the difference visible: there the enclosing
// directory is itself leaving, so a lock beside `notes` would recreate the
// project directory the restore expects to find absent.
func outermostDirectoryPaths(dirs []string) []string {
	outermost := make([]string, 0, len(dirs))
	kept := make(map[string]bool, len(dirs))
	for _, dir := range dirs {
		if kept[dir] {
			continue
		}
		nested := false
		for _, other := range dirs {
			if other == dir {
				continue
			}
			if pathWithinArtifactRoot(other, dir) {
				nested = true
				break
			}
		}
		if !nested {
			kept[dir] = true
			outermost = append(outermost, dir)
		}
	}
	return outermost
}
