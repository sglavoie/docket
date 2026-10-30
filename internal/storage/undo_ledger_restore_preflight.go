package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/transaction"
)

// prepareRestoreTransaction validates the complete selected entry before it
// stages anything, then journals every target image with the moved stack state.
// Artifact operations stay first and ledger.json is last, so recovery can always
// roll a crash forward to matching user bytes and visible history.
func (l *WorkspaceLedger) prepareRestoreTransaction(
	journal *transaction.Journal,
	lf *ledgerFile,
	entry LedgerEntry,
	idx int,
	redo bool,
) (*transaction.Manifest, error) {
	ops, err := l.preflightRestoreOps(lf, entry, redo)
	if err != nil {
		return nil, err
	}
	if redo {
		lf.RedoEntries = removeEntryAt(lf.RedoEntries, idx)
		lf.Entries = append(lf.Entries, entry)
	} else {
		lf.Entries = removeEntryAt(lf.Entries, idx)
		lf.RedoEntries = append(lf.RedoEntries, entry)
	}
	ledgerData, err := encodeLedgerFile(lf)
	if err != nil {
		return nil, err
	}
	ledgerOp := transaction.Op{
		Kind: transaction.OpWrite, Target: l.ledgerPath(), Data: ledgerData, Mode: FilePermissions,
	}
	if l.history == nil {
		manifest, prepareErr := journal.Prepare(append(ops, ledgerOp))
		if prepareErr != nil {
			return nil, fmt.Errorf("preparing ledger restore transaction: %w", prepareErr)
		}
		return manifest, nil
	}
	// An undo or redo is a new forward change to every other device, so it is
	// recorded as one, in the same manifest as the bytes it restores.
	kind := synclog.KindUndo
	if redo {
		kind = synclog.KindRedo
	}
	txnID := uuid.NewString()
	syncPlan, err := l.history.PlanLocal(kind, txnID, ops)
	if err != nil {
		return nil, fmt.Errorf("preparing sync history for ledger restore: %w", err)
	}
	ops = append(ops, syncPlan.Ops...)
	manifest, err := journal.PrepareWithID(txnID, append(ops, ledgerOp))
	if err != nil {
		return nil, fmt.Errorf("preparing ledger restore transaction: %w", err)
	}
	return manifest, nil
}

// retireDriftedEntry drops a selected entry whose restore was refused because an
// artifact no longer holds the image the entry left there, and reports both
// facts as one *RetiredEntryError. Nothing the ledger holds can explain that
// drift — a sync import and an external edit record no entry — so the image is
// never coming back, and an entry that can never restore would otherwise stay
// selected and wall off every older one behind it.
//
// Drift a pending entry does explain is left alone and reported as it always
// was: a scoped restore can select an entry older than one outside its scope
// that wrote the same file, and that entry is blocked, not dead. Any other
// failure, including a retirement that could not be written, is returned
// unchanged with the ledger as found.
func (l *WorkspaceLedger) retireDriftedEntry(
	journal *transaction.Journal,
	lf *ledgerFile,
	entry LedgerEntry,
	idx int,
	redo bool,
	cause error,
) error {
	var conflict *ArtifactConflictError
	if !errors.As(cause, &conflict) || driftExplainedByLedger(lf, entry, conflict.Path, redo) {
		return cause
	}
	if redo {
		lf.RedoEntries = removeEntryAt(lf.RedoEntries, idx)
	} else {
		lf.Entries = removeEntryAt(lf.Entries, idx)
	}
	ledgerData, err := encodeLedgerFile(lf)
	if err != nil {
		return cause
	}
	manifest, err := journal.Prepare([]transaction.Op{{
		Kind: transaction.OpWrite, Target: l.ledgerPath(), Data: ledgerData, Mode: FilePermissions,
	}})
	if err != nil {
		return cause
	}
	if err := commitJournal(journal, manifest); err != nil {
		return cause
	}
	l.ownWrites.record(manifest)
	l.gcBlobs(lf)
	return &RetiredEntryError{Entry: entry, Redo: redo, Conflict: conflict}
}

// driftExplainedByLedger reports whether an entry that must be restored before
// this one also wrote path: a newer one still on the undo stack, or, for a redo,
// an older one still waiting on the redo stack.
func driftExplainedByLedger(lf *ledgerFile, entry LedgerEntry, path string, redo bool) bool {
	path = filepath.Clean(path)
	writes := func(candidate LedgerEntry) bool {
		for _, image := range candidate.Artifacts {
			if filepath.Clean(image.Path) == path {
				return true
			}
		}
		return false
	}
	for _, candidate := range lf.Entries {
		if candidate.Seq > entry.Seq && writes(candidate) {
			return true
		}
	}
	if redo {
		for _, candidate := range lf.RedoEntries {
			if candidate.Seq < entry.Seq && writes(candidate) {
				return true
			}
		}
	}
	return false
}

// preflightRestoreOps validates both retained images and every current file or
// directory state before returning phased operations. File writes/deletes are
// emitted first. Directories that should be absent are removed deepest-first;
// directories that should exist are recreated by descendant file writes. Any
// overlapping drift aborts the grouped restore before Journal.Prepare stages bytes.
func (l *WorkspaceLedger) preflightRestoreOps(lf *ledgerFile, entry LedgerEntry, redo bool) ([]transaction.Op, error) {
	blobs, err := l.validateRestoreImages(entry)
	if err != nil {
		return nil, err
	}
	if err := l.checkTaskRemovalRestore(entry, blobs, redo); err != nil {
		return nil, err
	}

	fileOps := make([]transaction.Op, 0, len(entry.Artifacts))
	directoryDeletes := make(map[string]bool)
	fileDeletes := make(map[string]bool)
	for _, image := range entry.Artifacts {
		// A pending local entry still owns its place in the history. Rebase
		// only drift that came from outside the ledger, under the same locks
		// as the exact-image restore.
		rebaseTasks := l.isProjectTaskImage(image) && !driftExplainedByLedger(lf, entry, image.Path, redo)
		op, planned, planErr := preflightRestoreImage(image, redo, blobs, directoryDeletes, fileDeletes, rebaseTasks, len(entry.Artifacts) == 1)
		if planErr != nil {
			return nil, planErr
		}
		if planned {
			fileOps = append(fileOps, op)
		}
	}

	if entry.LegacySchemaVersion == undoLedgerSchemaVersionV1 {
		legacyDeletes, legacyErr := preflightLegacyAttachmentDirectoryDeletes(legacyAttachmentCleanupCandidates(entry, redo), fileDeletes)
		if legacyErr != nil {
			return nil, legacyErr
		}
		for _, dir := range legacyDeletes {
			directoryDeletes[dir] = true
		}
	}

	return appendDirectoryDeleteOps(fileOps, directoryDeletes), nil
}

func (l *WorkspaceLedger) validateRestoreImages(entry LedgerEntry) (map[string][]byte, error) {
	blobs := make(map[string][]byte)
	seen := make(map[string]bool, len(entry.Artifacts))
	for _, image := range entry.Artifacts {
		path := filepath.Clean(image.Path)
		if seen[path] {
			return nil, fmt.Errorf("storage: ledger entry contains duplicate artifact target %q", path)
		}
		seen[path] = true
		switch image.imageKind() {
		case ArtifactImageKindDirectory:
			if image.BeforeHash != "" || image.AfterHash != "" {
				return nil, fmt.Errorf("storage: ledger directory artifact %q cannot reference file blobs", path)
			}
		case ArtifactImageKindFile:
			if err := l.loadRestoreBlob(blobs, path, "before", image.BeforeExists, image.BeforeHash); err != nil {
				return nil, err
			}
			if err := l.loadRestoreBlob(blobs, path, "after", image.AfterExists, image.AfterHash); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("storage: ledger artifact %q has unknown image kind %q", path, image.Kind)
		}
	}
	return blobs, nil
}

func (l *WorkspaceLedger) loadRestoreBlob(blobs map[string][]byte, path, label string, exists bool, hash string) error {
	if !exists {
		if hash != "" {
			return fmt.Errorf("storage: ledger artifact %q has a %s hash for an absent image", path, label)
		}
		return nil
	}
	if hash == "" {
		return fmt.Errorf("storage: ledger artifact %q is missing its %s blob hash", path, label)
	}
	if _, ok := blobs[hash]; ok {
		return nil
	}
	data, err := l.readBlob(hash)
	if err != nil {
		return err
	}
	if actual := RawArtifactRevision(data); actual != hash {
		return fmt.Errorf("storage: undo blob %s failed its content hash check", shortRevision(hash))
	}
	blobs[hash] = data
	return nil
}

func preflightRestoreImage(
	image ArtifactImage,
	redo bool,
	blobs map[string][]byte,
	directoryDeletes map[string]bool,
	fileDeletes map[string]bool,
	rebaseTasks bool,
	allowMembershipChange bool,
) (transaction.Op, bool, error) {
	path := filepath.Clean(image.Path)
	expectedExists, expectedHash := image.AfterExists, image.AfterHash
	desiredExists, desiredHash := image.BeforeExists, image.BeforeHash
	if redo {
		expectedExists, expectedHash = image.BeforeExists, image.BeforeHash
		desiredExists, desiredHash = image.AfterExists, image.AfterHash
	}

	if image.imageKind() == ArtifactImageKindDirectory {
		actualExists, actualKind, err := readDirectoryState(path)
		if err != nil {
			return transaction.Op{}, false, err
		}
		expectedState := "<absent>"
		if expectedExists {
			expectedState = "directory"
		}
		if actualExists != expectedExists || (actualExists && actualKind != "directory") {
			return transaction.Op{}, false, &ArtifactConflictError{Path: path, Expected: expectedState, Actual: actualKind}
		}
		if !desiredExists {
			directoryDeletes[path] = true
		}
		return transaction.Op{}, false, nil
	}

	current, currentExists, actualKind, err := readRegularFileMaybe(path)
	if err != nil {
		return transaction.Op{}, false, fmt.Errorf("reading ledger restore target %q: %w", path, err)
	}
	actualHash := ""
	if currentExists && actualKind == "file" {
		actualHash = RawArtifactRevision(current)
	}
	if currentExists != expectedExists || (currentExists && (actualKind != "file" || actualHash != expectedHash)) {
		if rebaseTasks && currentExists && actualKind == "file" && expectedExists && desiredExists {
			if data, ok := rebaseTaskRestore(blobs[expectedHash], blobs[desiredHash], current, allowMembershipChange); ok {
				return transaction.Op{Kind: transaction.OpWrite, Target: path, Data: data, Mode: FilePermissions}, true, nil
			}
		}
		if actualKind != "file" {
			actualHash = actualKind
		}
		return transaction.Op{}, false, &ArtifactConflictError{Path: path, Expected: expectedHash, Actual: actualHash}
	}

	if desiredExists {
		return transaction.Op{Kind: transaction.OpWrite, Target: path, Data: blobs[desiredHash], Mode: FilePermissions}, true, nil
	}
	fileDeletes[path] = true
	return transaction.Op{Kind: transaction.OpDelete, Target: path}, true, nil
}

func appendDirectoryDeleteOps(fileOps []transaction.Op, directoryDeletes map[string]bool) []transaction.Op {
	dirs := make([]string, 0, len(directoryDeletes))
	for dir := range directoryDeletes {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool {
		leftDepth := strings.Count(filepath.Clean(dirs[i]), string(filepath.Separator))
		rightDepth := strings.Count(filepath.Clean(dirs[j]), string(filepath.Separator))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return dirs[i] < dirs[j]
	})
	for _, dir := range dirs {
		fileOps = append(fileOps, transaction.Op{Kind: transaction.OpDelete, Target: dir})
	}
	return fileOps
}

func readRegularFileMaybe(path string) ([]byte, bool, string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, "<absent>", nil
	}
	if err != nil {
		return nil, false, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, true, filesystemKind(info), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, true, "file", err
	}
	return data, true, "file", nil
}

func readDirectoryState(path string) (bool, string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, "<absent>", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("stating ledger directory restore target %q: %w", path, err)
	}
	return true, filesystemKind(info), nil
}

func filesystemKind(info os.FileInfo) string {
	switch {
	case info.IsDir():
		return "directory"
	case info.Mode().IsRegular():
		return "file"
	case info.Mode()&os.ModeSymlink != 0:
		return "symlink"
	default:
		return "non-regular"
	}
}

// legacyAttachmentCleanupCandidates identifies task attachment directories that
// a v1 file-only restore may need to remove. Safety is decided later from the
// complete current directory contents while the restore locks are held.
func legacyAttachmentCleanupCandidates(entry LedgerEntry, redo bool) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, image := range entry.Artifacts {
		if image.imageKind() != ArtifactImageKindFile {
			continue
		}
		desiredExists := image.BeforeExists
		if redo {
			desiredExists = image.AfterExists
		}
		if desiredExists {
			continue
		}
		dir := filepath.Dir(filepath.Clean(image.Path))
		if filepath.Base(filepath.Dir(dir)) != attachmentsDirName || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func attachmentDirectoryPaths(entry LedgerEntry) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, image := range entry.Artifacts {
		if image.imageKind() != ArtifactImageKindFile {
			continue
		}
		dir := filepath.Dir(filepath.Clean(image.Path))
		if filepath.Base(filepath.Dir(dir)) != attachmentsDirName || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

// preflightLegacyAttachmentDirectoryDeletes applies the v1 conservative rule:
// remove a task directory only when it currently contains at least one regular
// file and every entry is one of the files this restore already plans to delete.
// Unrelated siblings, symlinks, and nested directories always preserve it.
func preflightLegacyAttachmentDirectoryDeletes(candidates []string, fileDeletes map[string]bool) ([]string, error) {
	var deletes []string
	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading legacy attachment directory %q during restore preflight: %w", dir, err)
		}
		safe := len(entries) > 0
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("stating legacy attachment entry %q during restore preflight: %w", filepath.Join(dir, entry.Name()), err)
			}
			path := filepath.Join(dir, entry.Name())
			if !info.Mode().IsRegular() || !fileDeletes[path] {
				safe = false
				break
			}
		}
		if safe {
			deletes = append(deletes, dir)
		}
	}
	return deletes, nil
}
