package storage

import (
	"errors"
	"fmt"
	"os"

	"github.com/sglavoie/docket/internal/models"
)

// ConflictResolution names the three ways a persisted conflict can be resolved.
type ConflictResolution string

const (
	// ResolveUseLocal commits the caller's local edits for an explicitly
	// project-only bundle, discarding the remote divergence (subject to a re-merge
	// if remote advanced during resolution). Retry-only bundles reject it.
	ResolveUseLocal ConflictResolution = "local"
	// ResolveUseRemote adopts the current on-disk remote state, discarding local
	// edits. It writes nothing — the workspace already holds the remote content —
	// and simply clears the recorded conflict.
	ResolveUseRemote ConflictResolution = "remote"
	// ResolveUseFile commits a human-supplied merged file for an explicitly
	// project-only bundle after validating it and (if remote advanced) folding in
	// the newer remote via a re-merge. Retry-only bundles reject it.
	ResolveUseFile ConflictResolution = "file"
)

var (
	// ErrRemoteReconflict indicates that the remote advanced since the conflict was
	// recorded and a re-merge of the chosen resolution against the newer remote
	// produced fresh conflicts. The chosen resolution is not committed; a new
	// conflict bundle capturing the newer remote is recorded instead. Callers match
	// this with errors.Is and read the new id / conflicts from the returned result.
	ErrRemoteReconflict = errors.New("storage: remote changed during resolution and re-merge conflicts")
	// ErrConflictRetryRequired indicates that a conflict came from an aborted
	// commit whose complete transaction intent was not persisted. Applying only the
	// conflicting project would silently drop companion projects, artifacts, or
	// attachment operations, so local/file resolution fails closed. The operator
	// must adopt the current remote state to clear the bundle and retry the original
	// operation so the application can rebuild one complete ChangeSet.
	ErrConflictRetryRequired = errors.New("storage: conflict belongs to an aborted transaction; adopt remote and retry the original operation")
	// ErrConflictDocumentUnreadable indicates that the stored document a
	// resolution needs was written at a schema the current parser refuses, so it
	// cannot be committed or re-merged. Nothing migrates a bundle's documents when
	// the schema advances, which is why this is an expected outcome for an old
	// bundle rather than corruption. The resolutions that read no stored document
	// — adopting remote, committing a hand-merged file — stay available.
	ErrConflictDocumentUnreadable = errors.New("storage: stored conflict document cannot be read at the current schema")
)

// ConflictResolveResult reports the outcome of resolving a persisted conflict.
type ConflictResolveResult struct {
	// Committed is true when a new project revision was written to disk. Adopting
	// remote writes nothing, so it resolves with Committed false.
	Committed bool
	// Revision is the resulting canonical project revision on disk.
	Revision string
	// Remerged is true when a re-merge was performed to fold in the newer remote.
	Remerged bool
	// NewConflictID is set when the re-merge produced fresh conflicts: the chosen
	// resolution was not committed and a new bundle was recorded under this id.
	NewConflictID string
	// RetryRequired is true when the original commit was aborted without retaining
	// enough transaction intent to replay it safely. In that case no local or file
	// resolution is applied; adopting remote only clears the conflict record and
	// the caller must retry the original operation.
	RetryRequired bool
}

// RecordMergeConflict persists an explicitly project-only conflict bundle whose
// complete intent is represented by base/local/current. Commit-boundary callers
// must use MergeOrRecordConflict instead so an aborted transaction is marked as
// retry-only and cannot drop non-project side effects during resolution.
func RecordMergeConflict(store *ConflictStore, project *models.Project, base, local, current *ProjectDocument, conflicts []MergeConflict, action, context string) (string, error) {
	return recordMergeConflict(store, project, base, local, current, conflicts, action, context, false)
}

// recordMergeConflict persists a conflict and records whether resolving it must
// require a fresh operation retry. Commit-boundary conflicts set requiresRetry:
// the aborted ChangeSet may include companion projects, artifacts, attachment
// bytes, and ledger intent that the three project documents cannot reconstruct.
// Direct/legacy callers use RecordMergeConflict and retain project-only
// resolution behavior for bundles that never represented a larger transaction.
func recordMergeConflict(store *ConflictStore, project *models.Project, base, local, current *ProjectDocument, conflicts []MergeConflict, action, context string, requiresRetry bool) (string, error) {
	record := ConflictRecord{
		ProjectID:              project.ID,
		ProjectName:            project.Name,
		ProjectSlug:            project.Slug,
		ProjectPath:            project.Path,
		Action:                 action,
		Context:                context,
		BaseRevision:           documentRevision(base),
		LocalRevision:          documentRevision(local),
		RemoteRevision:         documentRevision(current),
		RequiresOperationRetry: &requiresRetry,
		Conflicts:              conflicts,
	}
	return store.Save(record, base, local, current)
}

// MergeOrRecordConflict runs a three-way merge and, when it conflicts, persists a
// conflict bundle and returns its id. On a clean merge it returns the merged
// result and an empty id; the caller may commit result.Merged. This is the
// intended integration seam for the commit boundary: attempt the merge on a
// stale-revision commit, commit the clean merge, and surface the recorded id when
// the merge cannot be resolved automatically.
func MergeOrRecordConflict(store *ConflictStore, project *models.Project, base, local, current *ProjectDocument, action, context string) (*ProjectMergeResult, string, error) {
	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		return nil, "", err
	}
	if !result.HasConflicts() {
		return result, "", nil
	}
	// This bundle represents a ChangeSet that the commit boundary aborted. The
	// three documents are not the complete transaction: artifacts, attachment
	// bytes, clean companion changesets, and undo intent may also have been present.
	// Mark it retry-only so resolution can never apply a misleading partial commit.
	id, err := recordMergeConflict(store, project, base, local, current, result.Conflicts, action, context, true)
	if err != nil {
		return result, "", err
	}
	return result, id, nil
}

// ResolveConflict resolves a persisted conflict by the chosen strategy and, on
// success, deletes it. It always re-reads the current on-disk document first so a
// remote edit that landed since the conflict was recorded is detected before
// anything is written ("recheck the remote revision before committing"):
//
//   - ResolveUseRemote adopts whatever is currently on disk and writes nothing.
//   - ResolveUseLocal / ResolveUseFile commit the chosen document. If the remote
//     advanced since the conflict was recorded, the chosen document is re-merged
//     against the newer remote; a clean re-merge commits, and a conflicting
//     re-merge records a fresh conflict and returns ErrRemoteReconflict without
//     writing.
//
// Conflicts recorded from an aborted commit carry RequiresOperationRetry because
// their bundle does not retain companion projects, artifacts, attachment bytes,
// or ledger intent. For those bundles local/file resolution is deliberately
// non-applying and returns ErrConflictRetryRequired; adopting remote clears the
// bundle without writing and tells the caller to retry the original operation.
// This prevents task metadata from being committed without its side effects.
//
// For explicitly project-only bundles, the commit itself re-verifies the revision
// under the project lock, so a second remote edit that races between this read and
// the commit is caught there and surfaces as a RevisionConflictError with the
// original conflict left intact. Pre-policy bundles with no marker fail closed as
// retry-only because their original transaction shape is unknown.
func ResolveConflict(store *ConflictStore, commit *CommitService, id string, resolution ConflictResolution, mergedFilePath string) (*ConflictResolveResult, error) {
	return resolveConflict(store, commit, id, resolution, mergedFilePath, nil)
}

// ResolveConflictText validates and commits a merge composed in an application.
// It uses the same retry policy and revision checks as file-based resolution.
func ResolveConflictText(store *ConflictStore, commit *CommitService, id, text string) (*ConflictResolveResult, error) {
	return resolveConflict(store, commit, id, ResolveUseFile, "", &text)
}

func resolveConflict(store *ConflictStore, commit *CommitService, id string, resolution ConflictResolution, mergedFilePath string, mergedText *string) (*ConflictResolveResult, error) {
	bundle, err := store.Load(id)
	if err != nil {
		return nil, err
	}
	record := bundle.Record
	project := &models.Project{
		ID:   record.ProjectID,
		Name: record.ProjectName,
		Slug: record.ProjectSlug,
		Path: record.ProjectPath,
	}

	currentDoc, err := LoadProjectDocument(project)
	if err != nil {
		return nil, fmt.Errorf("reloading current project state: %w", err)
	}
	remoteAdvanced := currentDoc.Revision != record.RemoteRevision

	result := &ConflictResolveResult{

		RetryRequired: record.OperationRetryRequired(),
	}

	// The persisted documents cannot reconstruct the complete aborted ChangeSet.
	// Refuse to commit a local/manual project document because doing so would drop
	// every non-project side effect. Adopting remote below remains safe: it writes
	// nothing, clears the stale record, and leaves the caller to retry the original
	// operation through the application layer.
	if record.OperationRetryRequired() && resolution != ResolveUseRemote {
		return result, ErrConflictRetryRequired
	}

	// Adopting remote keeps the on-disk state as-is: nothing to write, just clear
	// the recorded conflict.
	if resolution == ResolveUseRemote {
		if delErr := store.Delete(id); delErr != nil {
			return nil, delErr
		}
		result.Revision = currentDoc.Revision
		return result, nil
	}

	var resolved *ProjectDocument
	if mergedText != nil {
		resolved, err = parseResolvedText(*mergedText)
	} else {
		resolved, err = resolvedDocument(bundle, resolution, mergedFilePath)
	}
	if err != nil {
		return nil, err
	}

	// If the remote moved since the conflict was recorded, fold the newer remote
	// into the chosen resolution via a fresh three-way merge before committing.
	if remoteAdvanced {
		// The re-merge is a three-way merge, so it needs the recorded base. A base
		// the current parser refuses leaves no honest way to fold in the newer
		// remote, and committing the resolution as-is would silently discard it.
		if bundle.Base == nil || bundle.BaseUnreadable != "" {
			return nil, fmt.Errorf("%w: base document of conflict %s is required to re-merge against the newer remote: %s",
				ErrConflictDocumentUnreadable, id, baseUnreadableDetail(bundle))
		}
		remerge, mergeErr := MergeProjectDocuments(bundle.Base, resolved, currentDoc)
		if mergeErr != nil {
			return nil, fmt.Errorf("re-merging against newer remote: %w", mergeErr)
		}
		result.Remerged = true
		if remerge.HasConflicts() {
			// The old bundle's remote is stale; replace it with one capturing the
			// newer remote so the operator resolves against current reality.
			newID, recErr := RecordMergeConflict(store, project, bundle.Base, resolved, currentDoc, remerge.Conflicts, record.Action, record.Context)
			if recErr != nil {
				return nil, recErr
			}
			if delErr := store.Delete(id); delErr != nil {
				return nil, delErr
			}
			result.NewConflictID = newID
			return result, fmt.Errorf("%w: recorded new conflict %s", ErrRemoteReconflict, newID)
		}
		resolved = remerge.Merged
	}

	commitProject := projectFromDocument(project, resolved)
	commitResult, err := commit.Commit(ChangeSet{
		Project:          commitProject,
		ExpectedRevision: currentDoc.Revision,
		// The chosen resolution is already a complete lossless document. Use it
		// as the preservation source so a locally selected opaque block is not
		// replaced by the current remote document during the write.
		Base:        resolved,
		UndoAction:  resolveUndoAction(resolution),
		UndoContext: fmt.Sprintf("resolve conflict %s for %s", id, record.ProjectName),
	})
	if err != nil {
		// A RevisionConflictError here means a second remote edit raced between the
		// read above and the commit; leave the conflict recorded for a retry.
		return nil, err
	}

	if err := store.Delete(id); err != nil {
		return nil, err
	}
	result.Committed = true
	result.Revision = commitResult.Revision
	return result, nil
}

// baseUnreadableDetail names why the base document is unusable. A recorded
// reason is the interesting case; an absent file is the degenerate one a
// partially-written bundle produces.
func baseUnreadableDetail(bundle *ConflictBundle) string {
	if bundle.BaseUnreadable != "" {
		return bundle.BaseUnreadable
	}
	return "no base document is stored"
}

// resolvedDocument produces the document a local/file resolution intends to
// commit. For a file resolution it parses and validates the supplied merged file
// before returning it, so a malformed or invariant-violating hand-merge is
// rejected before any commit.
func resolvedDocument(bundle *ConflictBundle, resolution ConflictResolution, mergedFilePath string) (*ProjectDocument, error) {
	switch resolution {
	case ResolveUseLocal:
		if bundle.LocalUnreadable != "" {
			return nil, fmt.Errorf("%w: local document of conflict %s: %s",
				ErrConflictDocumentUnreadable, bundle.Record.ID, bundle.LocalUnreadable)
		}
		if bundle.Local == nil {
			return nil, fmt.Errorf("conflict %s has no stored local document", bundle.Record.ID)
		}
		return bundle.Local, nil
	case ResolveUseFile:
		if mergedFilePath == "" {
			return nil, fmt.Errorf("a merged file path is required for --use file")
		}
		content, err := os.ReadFile(mergedFilePath)
		if err != nil {
			return nil, fmt.Errorf("reading merged file: %w", err)
		}
		return parseResolvedText(string(content))
	default:
		return nil, fmt.Errorf("unknown conflict resolution %q", resolution)
	}
}

func parseResolvedText(text string) (*ProjectDocument, error) {
	doc, err := ParseProjectDocument(text)
	if err != nil {
		return nil, fmt.Errorf("parsing merged file: %w", err)
	}
	if conflicts := auditMergedDocument(doc); len(conflicts) > 0 {
		return nil, fmt.Errorf("merged file failed validation: %s", conflicts[0].Detail)
	}
	return doc, nil
}

// projectFromDocument reconstructs a models.Project ready for commit from a
// resolved document, keeping the live project's identity (id, name, slug, path)
// and taking canonical task content from the document. Legacy
// project sections/colors are decode-only and receive runtime defaults.
func projectFromDocument(identity *models.Project, doc *ProjectDocument) *models.Project {
	return &models.Project{
		ID:       identity.ID,
		Name:     identity.Name,
		Slug:     identity.Slug,
		Path:     identity.Path,
		Tasks:    doc.Tasks,
		Comments: models.CloneProjectComments(doc.Comments),
	}
}

// documentRevision returns a document's revision, tolerating a nil document.
func documentRevision(doc *ProjectDocument) string {
	if doc == nil {
		return ""
	}
	if doc.Revision != "" {
		return doc.Revision
	}
	return doc.ComputeRevision()
}

// resolveUndoAction describes the resolution for the undo ledger.
func resolveUndoAction(resolution ConflictResolution) string {
	return "conflict resolve (" + string(resolution) + ")"
}
