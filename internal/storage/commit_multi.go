package storage

import (
	"fmt"
	"os"
	"sort"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/transaction"
	"github.com/sglavoie/tt/internal/util"
)

func (s *CommitService) preflightChangesets(changesets []ChangeSet) ([]*ProjectDocument, []RecordedConflict, error) {
	// mergedDocs[i], when non-nil, is the three-way-merged document that replaces
	// changeset i's local intent because its expected revision was stale but the
	// concurrent edits merged cleanly. Phase 3 writes it verbatim instead of
	// re-rendering the caller's local document.
	mergedDocs := make([]*ProjectDocument, len(changesets))
	// recorded accumulates every project whose stale-revision merge produced true
	// conflicts. A non-empty set aborts the whole commit after every conflict is
	// durably recorded, so no project or artifact is written.
	var recorded []RecordedConflict

	// Project preconditions are deliberately a separate first phase. They guard
	// the complete application-level read set and must expire atomically before
	// the ordinary ExpectedRevision path can attempt a merge or record a durable
	// conflict bundle for any changeset.
	if err := s.validateProjectRevisionPreconditions(changesets); err != nil {
		return nil, nil, err
	}

	// Phase 1: preflight every project's expected revision under the held locks.
	// No bytes are written yet, so a conflict here leaves every project untouched.
	for i := range changesets {
		cs := changesets[i]
		project := cs.Project
		currentDoc, currentRevision, err := s.currentDocAndRevision(tasksFilePath(project.Path), project)
		if err != nil {
			return nil, nil, err
		}
		switch {
		case cs.ExpectedRevision == "" || cs.ExpectedRevision == currentRevision:
			// In sync (or the caller opted out of the check): nothing to merge.
		case s.conflicts != nil && cs.Base != nil && currentDoc != nil:
			// Stale revision with a real merge base and a durable conflict store:
			// three-way merge the loaded base, the caller's local intent, and the
			// current on-disk document under the held locks. A clean non-overlapping
			// merge replaces the local intent and commits against the current
			// revision; a true conflict is durably recorded as retry-only and aborts
			// the whole set. Retry-only is required because this project bundle cannot
			// represent the complete ChangeSet (including artifacts, attachment bytes,
			// clean companion projects, and undo intent).
			local, mergeErr := CanonicalProjectDocument(project, cs.Base)
			if mergeErr != nil {
				return nil, nil, fmt.Errorf("rendering local document for merge of %s: %w", project.Name, mergeErr)
			}
			merge, conflictID, mergeErr := MergeOrRecordConflict(s.conflicts, project, cs.Base, local, currentDoc, cs.UndoAction, cs.UndoContext)
			if mergeErr != nil {
				return nil, nil, fmt.Errorf("merging stale project %s: %w", project.Name, mergeErr)
			}
			if conflictID != "" {
				recorded = append(recorded, RecordedConflict{
					ProjectID:   project.ID,
					ProjectName: project.Name,
					ConflictID:  conflictID,
				})
				continue
			}
			// Re-parse the merged rendering so the committed revision is a fixed point
			// (reload-stable): it equals exactly what the next LoadProjectDocument sees.
			mergedContent, renderErr := merge.Merged.WriteCurrent()
			if renderErr != nil {
				return nil, nil, fmt.Errorf("rendering merged document for %s: %w", project.Name, renderErr)
			}
			canonical, canonErr := ParseProjectDocument(mergedContent)
			if canonErr != nil {
				return nil, nil, fmt.Errorf("canonicalizing merged document for %s: %w", project.Name, canonErr)
			}
			// The merge folded in the current on-disk state, so mergedDocs[i] is what
			// Phase 3 writes; the write is safe under the still-held project lock (no
			// further revision check is needed, the merge already reconciled current).
			mergedDocs[i] = canonical
		default:
			// Stale with no merge capability (nil base, corrupt or absent current doc,
			// or no conflict store): abort as before so the caller reloads and retries.
			return nil, nil, &RevisionConflictError{
				ProjectID:   project.ID,
				ProjectName: project.Name,
				Expected:    cs.ExpectedRevision,
				Actual:      currentRevision,
			}
		}
		// Validate every artifact's optimistic revision under the same held locks,
		// before any bytes are staged, so a stale config artifact aborts the whole
		// set without touching disk.
		if err := validateArtifactRevisions(cs.Artifacts); err != nil {
			return nil, nil, err
		}
	}
	return mergedDocs, recorded, nil
}

func (s *CommitService) validateProjectRevisionPreconditions(changesets []ChangeSet) error {
	for i := range changesets {
		for j := range changesets[i].ProjectPreconditions {
			precondition := changesets[i].ProjectPreconditions[j]
			identity := &models.Project{
				ID:   precondition.ProjectID,
				Name: precondition.ProjectName,
				Path: precondition.Path,
			}
			_, actual, err := s.currentDocAndRevision(tasksFilePath(precondition.Path), identity)
			if err != nil {
				return err
			}
			if actual != precondition.ExpectedRevision {
				return &RevisionConflictError{
					ProjectID:   precondition.ProjectID,
					ProjectName: precondition.ProjectName,
					Expected:    precondition.ExpectedRevision,
					Actual:      actual,
				}
			}
		}
	}
	return nil
}

// commitMultiLocked performs the full preflight, staging, and commit for every
// changeset while the caller holds every affected project's exclusive lock. It
// verifies all expected revisions before touching any file, so a stale revision
// anywhere aborts the whole set before any backup or staging is written.
func (s *CommitService) commitMultiLocked(journal *transaction.Journal, changesets []ChangeSet) ([]*CommitResult, error) {
	mergedDocs, recorded, preflightErr := s.preflightChangesets(changesets)
	if preflightErr != nil {
		return nil, preflightErr
	}

	// A single unresolved merge aborts the whole set: nothing is written, but every
	// conflict has been durably recorded for later resolution through the shared
	// resolution service.
	if len(recorded) > 0 {
		return nil, &CommitConflictError{Conflicts: recorded}
	}

	// Build every project write plus every artifact as one transaction so
	// the whole cross-project mutation is atomic. The markdown content is
	// rendered in the canonical live schema before staging.
	ops := make([]transaction.Op, 0, len(changesets))
	// dirsToCreate collects target parent directories that must exist before the
	// journal renames staged bytes into place: attachment move/copy destinations
	// and any artifact write whose parent (e.g. a project's notes/ subdirectory)
	// does not yet exist. They are created only after Prepare validates every
	// target and before Commit applies the ops, so a rejected target never leaves
	// an empty directory behind.
	var dirsToCreate []string
	// When a ledger is attached, capture the before/after image of every file
	// each changeset touches under the held locks, before the transaction applies
	// (so before-images are the real pre-mutation bytes and an attachment delete
	// or move can be reversed). recordAction/recordContext take the first non-empty
	// description across the set — the whole multi-commit is one ledger entry.
	var ledgerArtifacts []LedgerArtifact
	var ledgerProjectIDs []string
	var recordAction, recordContext string
	committedDocs := make([]*ProjectDocument, len(changesets))
	for i := range changesets {
		cs := changesets[i]
		project := cs.Project
		// A stale changeset that merged cleanly writes the merged document; every
		// other changeset renders the caller's local intent over its base.
		committedDoc := mergedDocs[i]
		if committedDoc == nil {
			var err error
			committedDoc, err = CanonicalProjectDocument(project, cs.Base)
			if err != nil {
				return nil, fmt.Errorf("rendering project %s: %w", project.Name, err)
			}
		}
		content, err := committedDoc.WriteCurrent()
		if err != nil {
			return nil, fmt.Errorf("rendering project %s: %w", project.Name, err)
		}
		committedDocs[i] = committedDoc
		if s.ledger != nil {
			arts, captureErr := captureLedgerArtifacts(cs, content)
			if captureErr != nil {
				return nil, fmt.Errorf("capturing undo images for project %s: %w", project.Name, captureErr)
			}
			ledgerArtifacts = append(ledgerArtifacts, arts...)
			ledgerProjectIDs = append(ledgerProjectIDs, ledgerProjectID(project))
			if recordAction == "" {
				recordAction = cs.UndoAction
			}
			if recordContext == "" {
				recordContext = cs.UndoContext
			}
		}
		ops = append(ops, transaction.Op{
			Kind:   transaction.OpWrite,
			Target: tasksFilePath(project.Path),
			Data:   []byte(content),
			Mode:   FilePermissions,
		})
		artifactOps, artifactDirs, err := expandArtifactChanges(cs.Artifacts)
		if err != nil {
			return nil, err
		}
		ops = append(ops, artifactOps...)
		dirsToCreate = append(dirsToCreate, artifactDirs...)
		// Resolve this changeset's attachment side effects into transaction ops
		// under the held locks. A resolution failure (missing copy source, occupied
		// move destination, nested directory) aborts the whole commit here — before
		// Prepare stages any bytes — so no project or artifact is written.
		attachOps, dirs, attachErr := resolveAttachmentOps(cs.AttachmentOps)
		if attachErr != nil {
			return nil, fmt.Errorf("resolving attachment operations for project %s: %w", project.Name, attachErr)
		}
		ops = append(ops, attachOps...)
		dirsToCreate = append(dirsToCreate, dirs...)
	}

	var ledgerRecord *LedgerRecord
	if s.ledger != nil {
		ledgerRecord = &LedgerRecord{
			Action:     recordAction,
			Context:    recordContext,
			ProjectIDs: ledgerProjectIDs,
			Artifacts:  ledgerArtifacts,
		}
	}
	manifest, ledgerPlan, err := s.prepareTransaction(journal, ops, ledgerRecord)
	if err != nil {
		return nil, err
	}
	// Create every collected target parent directory (attachment destinations and
	// artifact write parents) now that Prepare validated every target and before
	// Commit applies the operations, so a rejected target never leaves an empty
	// directory and the rename-into-place never fails on a missing parent.
	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(dir, DirPermissions); err != nil {
			return nil, fmt.Errorf("creating commit target directory: %w", err)
		}
	}
	if err := journal.Commit(manifest); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}
	if ledgerPlan != nil {
		// Cleanup is deliberately post-commit and best-effort. A crash can leak an
		// unreferenced content-addressed blob, but the journal has already made the
		// project, companion artifacts, retained blobs, and ledger entry durable.
		s.ledger.gcBlobs(ledgerPlan.file)
	}

	results := make([]*CommitResult, len(changesets))
	for i := range changesets {
		results[i] = &CommitResult{
			Revision:      committedDocs[i].Revision,
			TransactionID: manifest.ID,
		}
	}
	return results, nil
}

func ledgerProjectID(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.ID != "" {
		return project.ID
	}
	return project.Slug
}

// withOrderedFileLocks acquires every distinct lock path in sorted order and runs
// fn while all are held. Sorting gives a deterministic global acquisition order so
// two concurrent multi-commits touching an overlapping set of projects can never
// deadlock, and de-duplication avoids re-locking the same path in-process (flock
// is per-open-file-description, so a second lock on the same path from the same
// process would deadlock against the first).
func withOrderedFileLocks(lockPaths []string, fn func() error) error {
	seen := make(map[string]bool, len(lockPaths))
	ordered := make([]string, 0, len(lockPaths))
	for _, p := range lockPaths {
		identity := artifactPathIdentity(p)
		if seen[identity] {
			continue
		}
		seen[identity] = true
		ordered = append(ordered, identity)
	}
	sort.Strings(ordered)
	return acquireLocksInOrder(ordered, fn)
}

// acquireLocksInOrder recursively acquires each lock in paths (already sorted and
// de-duplicated) and runs fn once every lock is held.
func acquireLocksInOrder(paths []string, fn func() error) error {
	if len(paths) == 0 {
		return fn()
	}
	return util.WithFileLock(paths[0], true, func() error {
		return acquireLocksInOrder(paths[1:], fn)
	})
}

// currentDocAndRevision reads and parses the project's current tasks.md,
// returning the parsed document (the three-way merge's "current" input) and its
// canonical revision, or (nil, "") when the file is absent or unparseable.
// A nil document routes a stale commit to a plain
// RevisionConflictError since it has no mergeable current, matching the legacy
// save path's tolerance of an unparseable on-disk file.
func (s *CommitService) currentDocAndRevision(tasksPath string, project *models.Project) (*ProjectDocument, string, error) {
	oldContent, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to read tasks file: %w", err)
	}
	oldDoc, parseErr := ParseProjectDocument(string(oldContent))
	if parseErr != nil {
		logging.Warn("could not read current document for %s: %v", project.Name, parseErr)
		return nil, "", nil
	}
	return oldDoc, oldDoc.Revision, nil
}
