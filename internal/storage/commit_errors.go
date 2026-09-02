package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Typed conflict and recovery errors surfaced by the commit boundary. Callers
// classify a failure with errors.Is rather than matching message text.
var (
	// ErrStaleRevision indicates the on-disk project changed since the caller
	// loaded it: the expected revision no longer matches what is on disk, so the
	// changeset is aborted rather than clobbering the concurrent change. The
	// caller should reload and retry (or, later, three-way merge).
	ErrStaleRevision = errors.New("storage: project revision is stale; reload and retry")
	// ErrCommitBlocked indicates commit is refused because startup crash recovery
	// quarantined one or more irrecoverable transactions. The workspace stays
	// blocked until a human resolves the quarantine.
	ErrCommitBlocked = errors.New("storage: commit blocked by quarantined transactions")
	// ErrStaleArtifact indicates a configuration artifact changed on disk since the
	// caller loaded it: its ExpectedRevision no longer matches the current raw hash
	// (or an expected-absent artifact now exists), so the commit is aborted rather
	// than clobbering the concurrent write. It is the artifact-level counterpart to
	// ErrStaleRevision and the caller should reload the config domain and retry.
	ErrStaleArtifact = errors.New("storage: artifact revision is stale; reload and retry")
	// ErrMergeConflict indicates a stale-revision commit was three-way merged against
	// the current on-disk state and produced conflicts that cannot be resolved
	// automatically. The conflict bundle(s) have been durably recorded and marked
	// as requiring a fresh operation retry because they intentionally do not retain
	// the aborted transaction's companion projects, artifacts, attachment bytes, or
	// ledger intent. The whole commit was aborted with no project or artifact write.
	// Callers match this class with errors.Is and read the recorded ids from a
	// *CommitConflictError.
	ErrMergeConflict = errors.New("storage: commit produced unresolved merge conflicts recorded for resolution")
)

// RecordedConflict names one project whose stale-revision merge could not be
// resolved automatically and the durable conflict bundle recorded for it.
type RecordedConflict struct {
	ProjectID   string
	ProjectName string
	// ConflictID is the id of the durably recorded conflict bundle; resolve it
	// through the shared resolution service (tt conflict resolve / the TUI).
	ConflictID string
}

// CommitConflictError is the concrete unresolved-merge conflict returned when one
// or more stale-revision changesets three-way merged into true conflicts. Every
// listed conflict has already been durably recorded (so a later resolution can
// reconcile it) and the whole commit was aborted with no writes. It unwraps to
// ErrMergeConflict so callers can match the class while inspecting the recorded
// ids and structured conflicts.
type CommitConflictError struct {
	Conflicts []RecordedConflict
}

func (e *CommitConflictError) Error() string {
	if len(e.Conflicts) == 1 {
		c := e.Conflicts[0]
		name := c.ProjectName
		if name == "" {
			name = c.ProjectID
		}
		return fmt.Sprintf("storage: project %s has unresolved merge conflicts (recorded as %s); adopt remote, then retry the original operation", name, c.ConflictID)
	}
	ids := make([]string, 0, len(e.Conflicts))
	for _, c := range e.Conflicts {
		ids = append(ids, c.ConflictID)
	}
	return fmt.Sprintf("storage: %d projects have unresolved merge conflicts (recorded as %s); adopt remote, then retry the original operation", len(e.Conflicts), strings.Join(ids, ", "))
}

func (e *CommitConflictError) Unwrap() error { return ErrMergeConflict }

// ArtifactConflictError is the concrete stale-artifact conflict returned when an
// ArtifactChange's ExpectedRevision does not match the current on-disk raw hash
// (or an expected-absent artifact already exists). It unwraps to ErrStaleArtifact
// so callers can match the class while inspecting the specific path and the
// diverging revisions.
type ArtifactConflictError struct {
	Path     string
	Expected string
	Actual   string
}

func (e *ArtifactConflictError) Error() string {
	return fmt.Sprintf("storage: artifact %s changed on disk (expected revision %s, found %s): reload and retry", filepath.Base(e.Path), shortRevision(e.Expected), shortRevision(e.Actual))
}

func (e *ArtifactConflictError) Unwrap() error { return ErrStaleArtifact }

// RevisionConflictError is the concrete stale-revision conflict returned when a
// changeset's expected revision does not match the current on-disk revision. It
// unwraps to ErrStaleRevision so callers can match the class while still
// inspecting the specific project and the diverging revisions.
type RevisionConflictError struct {
	ProjectID   string
	ProjectName string
	Expected    string
	Actual      string
}

func (e *RevisionConflictError) Error() string {
	name := e.ProjectName
	if name == "" {
		name = e.ProjectID
	}
	if name == "" {
		name = "<unknown>"
	}
	return fmt.Sprintf("storage: project %s changed on disk (expected revision %s, found %s): reload and retry", name, shortRevision(e.Expected), shortRevision(e.Actual))
}

func (e *RevisionConflictError) Unwrap() error { return ErrStaleRevision }

// shortRevision trims a hex revision to a readable prefix for error messages,
// leaving short or empty revisions untouched.
func shortRevision(rev string) string {
	if rev == "" {
		return "<none>"
	}
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}
