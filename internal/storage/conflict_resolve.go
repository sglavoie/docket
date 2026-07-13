package storage

import (
	"errors"
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/models"
)

// ConflictResolution names the three ways a persisted conflict can be resolved.
type ConflictResolution string

const (
	// ResolveUseLocal commits the caller's local edits, discarding the remote
	// divergence (subject to a re-merge if remote advanced during resolution).
	ResolveUseLocal ConflictResolution = "local"
	// ResolveUseRemote adopts the current on-disk remote state, discarding local
	// edits. It writes nothing — the workspace already holds the remote content —
	// and simply clears the recorded conflict.
	ResolveUseRemote ConflictResolution = "remote"
	// ResolveUseFile commits a human-supplied merged file after validating it and
	// (if remote advanced) folding in the newer remote via a re-merge.
	ResolveUseFile ConflictResolution = "file"
)

// ErrRemoteReconflict indicates that the remote advanced since the conflict was
// recorded and a re-merge of the chosen resolution against the newer remote
// produced fresh conflicts. The chosen resolution is not committed; a new
// conflict bundle capturing the newer remote is recorded instead. Callers match
// this with errors.Is and read the new id / conflicts from the returned result.
var ErrRemoteReconflict = errors.New("storage: remote changed during resolution and re-merge conflicts")

// ConflictResolveResult reports the outcome of resolving a persisted conflict.
type ConflictResolveResult struct {
	// ConflictID is the id that was resolved (and, on success, deleted).
	ConflictID string
	// Resolution is the chosen strategy.
	Resolution ConflictResolution
	// Committed is true when a new project revision was written to disk. Adopting
	// remote writes nothing, so it resolves with Committed false.
	Committed bool
	// Revision is the resulting canonical project revision on disk.
	Revision string
	// RemoteAdvanced is true when the on-disk remote had moved past the revision
	// captured when the conflict was recorded.
	RemoteAdvanced bool
	// Remerged is true when a re-merge was performed to fold in the newer remote.
	Remerged bool
	// NewConflictID is set when the re-merge produced fresh conflicts: the chosen
	// resolution was not committed and a new bundle was recorded under this id.
	NewConflictID string
	// Conflicts holds the fresh conflicts when NewConflictID is set.
	Conflicts []MergeConflict
}

// RecordMergeConflict persists a conflict bundle for a project whose commit hit a
// three-way merge it could not resolve automatically. base/local/current are the
// three merge inputs and conflicts are the structured conflicts the merge
// produced. It returns the generated conflict id.
func RecordMergeConflict(store *ConflictStore, project *models.Project, base, local, current *ProjectDocument, conflicts []MergeConflict, action, context string) (string, error) {
	record := ConflictRecord{
		ProjectID:      project.ID,
		ProjectName:    project.Name,
		ProjectSlug:    project.Slug,
		ProjectPath:    project.Path,
		Action:         action,
		Context:        context,
		BaseRevision:   documentRevision(base),
		LocalRevision:  documentRevision(local),
		RemoteRevision: documentRevision(current),
		Conflicts:      conflicts,
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
	id, err := RecordMergeConflict(store, project, base, local, current, result.Conflicts, action, context)
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
// The commit itself re-verifies the revision under the project lock, so a second
// remote edit that races between this read and the commit is caught there and
// surfaces as a RevisionConflictError with the original conflict left intact.
func ResolveConflict(store *ConflictStore, commit *CommitService, id string, resolution ConflictResolution, mergedFilePath string) (*ConflictResolveResult, error) {
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
		ConflictID:     id,
		Resolution:     resolution,
		RemoteAdvanced: remoteAdvanced,
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

	resolved, err := resolvedDocument(bundle, resolution, mergedFilePath)
	if err != nil {
		return nil, err
	}

	// If the remote moved since the conflict was recorded, fold the newer remote
	// into the chosen resolution via a fresh three-way merge before committing.
	if remoteAdvanced {
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
			result.Conflicts = remerge.Conflicts
			return result, fmt.Errorf("%w: recorded new conflict %s", ErrRemoteReconflict, newID)
		}
		resolved = remerge.Merged
	}

	commitProject := projectFromDocument(project, resolved)
	commitResult, err := commit.Commit(ChangeSet{
		Project:          commitProject,
		ExpectedRevision: currentDoc.Revision,
		Opaque:           resolved.OpaqueBlocks,
		UndoAction:       resolveUndoAction(resolution),
		UndoContext:      fmt.Sprintf("resolve conflict %s for %s", id, record.ProjectName),
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

// resolvedDocument produces the document a local/file resolution intends to
// commit. For a file resolution it parses and validates the supplied merged file
// before returning it, so a malformed or invariant-violating hand-merge is
// rejected before any commit.
func resolvedDocument(bundle *ConflictBundle, resolution ConflictResolution, mergedFilePath string) (*ProjectDocument, error) {
	switch resolution {
	case ResolveUseLocal:
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
		doc, err := ParseProjectDocument(string(content))
		if err != nil {
			return nil, fmt.Errorf("parsing merged file: %w", err)
		}
		if conflicts := auditMergedDocument(doc); len(conflicts) > 0 {
			return nil, fmt.Errorf("merged file failed validation: %s", conflicts[0].Detail)
		}
		return doc, nil
	default:
		return nil, fmt.Errorf("unknown conflict resolution %q", resolution)
	}
}

// projectFromDocument reconstructs a models.Project ready for commit from a
// resolved document, keeping the live project's identity (id, name, slug, path)
// and taking all content (tasks, sections, colors, tags) from the document.
func projectFromDocument(identity *models.Project, doc *ProjectDocument) *models.Project {
	return &models.Project{
		ID:            identity.ID,
		Name:          identity.Name,
		Slug:          identity.Slug,
		Path:          identity.Path,
		Tasks:         doc.Tasks,
		Sections:      doc.Sections,
		SectionColors: doc.SectionColors,
		Tags:          doc.Tags,
		PinnedTags:    doc.PinnedTags,
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
