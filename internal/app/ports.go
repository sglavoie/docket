// Package app is the presentation-neutral application service layer for Docket. It
// exposes narrow task-mutation use cases (capture, edit, state transition, due
// date, priority, tag, pin, deferral, park, completion) that validate input,
// apply domain rules through the models package, and emit exactly one storage
// ChangeSet committed through the repository boundary. It imports neither Cobra
// nor Bubble Tea. The TUI is the only adapter driving these use cases; it
// translates the presentation-neutral results and typed errors into its own
// output. The Cobra commands hold only the launcher and maintenance paths and
// never reach this layer. Every dependency the layer needs — the mutation
// repository, a clock, and an ID generator — is an injected interface so the
// use cases are deterministic and testable against an in-memory repository.
package app

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

// OpsCommitter is the optional capability a Repository exposes to commit a set of
// raw file operations plus revision-checked artifacts as one atomic journaled
// transaction. Project rename and workspace maintenance use this boundary when
// directory and config changes must land together. Keeping artifacts typed
// preserves their optimistic-concurrency contracts instead of flattening them into
// unchecked writes.
type OpsCommitter interface {
	CommitOps(ops []transaction.Op, artifacts []storage.ArtifactChange) error
}

// UndoableOpsCommitter extends OpsCommitter for user-facing project lifecycle
// changes. Implementations commit the raw operations, artifacts, and workspace
// ledger entry as one transaction.
type UndoableOpsCommitter interface {
	CommitOpsTransaction(request storage.OpsCommitRequest) (string, error)
}

// ArtifactCommitter is the repository capability used by workspace-note
// mutations. It commits only the note artifacts: no project tasks.md rewrite is
// manufactured for a note-only action. The storage implementation journals the
// artifacts and their workspace-ledger record atomically.
type ArtifactCommitter interface {
	CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error)
}

// CensusProject is one project of a strict workspace census, paired with the
// canonical revision and the exact lossless document it was read at. The trio
// is what lets a whole-workspace mutation both mutate the project and guard the
// state it was derived from at the commit boundary.
type CensusProject struct {
	Project  *models.Project
	Revision string
	Document *storage.ProjectDocument
}

// WorkspaceCensus is a complete, strict read of every unarchived project —
// parked ones included, in every task state — taken together with the identity
// and revision of the catalog that enumerated them. It is deliberately not a
// Snapshot: Snapshot skips a project it cannot read so one broken document
// never blocks an unrelated single-task edit, while a census that silently
// omitted a project would let a whole-workspace rename report and commit an
// incomplete result. Every read failure fails the census instead.
type WorkspaceCensus struct {
	// Projects are the enumerated projects in catalog order.
	Projects []CensusProject
	// CatalogPath is the artifact the enumeration came from, guarded read-only at
	// the commit boundary so a concurrent catalog write expires the census.
	CatalogPath string
	// CatalogRevision is that artifact's raw revision at census time.
	CatalogRevision string
}

// WorkspaceCensusSource is the optional repository capability behind
// whole-workspace vocabulary mutations. Only the production filesystem
// repository implements it; a repository without it refuses the operation by
// name rather than silently renaming across a partial workspace.
type WorkspaceCensusSource interface {
	WorkspaceCensus() (WorkspaceCensus, error)
}

// Clock supplies wall-clock time to the use cases. Injecting it keeps
// time-derived behavior (deferral targets, due dates) deterministic under test.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// SystemClock returns a Clock backed by the real wall clock.
func SystemClock() Clock { return systemClock{} }

// IDGenerator mints stable task identifiers for newly captured tasks.
type IDGenerator interface {
	NewID() string
}

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.New().String() }

// UUIDGenerator returns an IDGenerator that produces random UUID v4 strings,
// matching the identifiers the legacy CLI mints for new tasks.
func UUIDGenerator() IDGenerator { return uuidGenerator{} }

// TaskRef locates a task and the project that owns it inside a snapshot. It is
// the value the read model returns when resolving a task by its UUID.
type TaskRef struct {
	Task    *models.Task
	Project *models.Project
	// Base is the exact lossless project document retained when the snapshot was
	// loaded. Mutations carry it into storage.ChangeSet so opaque header content
	// survives and stale commits have a real three-way merge base.
	Base *storage.ProjectDocument
}

// TaskLocator identifies one task without relying on workspace-wide UUID
// uniqueness. Cross-project presentation layers must retain both fields until
// the application snapshot resolves the task.
type TaskLocator struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
}

func (l TaskLocator) Valid() bool { return l.ProjectID != "" && l.TaskID != "" }

// TaskLookupStatus describes the result of resolving an unqualified task UUID
// across the workspace. Ambiguous IDs deliberately carry no TaskRef: callers
// must provide a TaskLocator instead of relying on project iteration order.
type TaskLookupStatus uint8

const (
	TaskLookupMissing TaskLookupStatus = iota
	TaskLookupUnique
	TaskLookupAmbiguous
)

// Snapshot is the revision-consistent read model the application layer consumes
// before a mutation: it resolves task and project references and reports the
// canonical revision each project was loaded at, which the use cases capture as
// the optimistic-concurrency expectation on the emitted ChangeSet.
type Snapshot interface {
	// ProjectByID returns the loaded project with the given ID, or nil.
	ProjectByID(projectID string) *models.Project
	// LookupTaskID resolves an unqualified task UUID only when exactly one loaded
	// project owns it.
	LookupTaskID(taskID string) (TaskRef, TaskLookupStatus)
	// LookupLocator resolves a task within an explicit owning project.
	LookupLocator(locator TaskLocator) (TaskRef, bool)
	// Revision returns the canonical content revision a project was loaded at,
	// or "" when the project is not part of the snapshot.
	Revision(projectID string) string
	// Document returns the exact lossless project document retained at load time,
	// or nil when the snapshot has no document for the project (legacy/test-only
	// snapshots may omit it).
	Document(projectID string) *storage.ProjectDocument
	// Projects returns every loaded project.
	Projects() []*models.Project
	// UnavailableProjectIDs names the cataloged projects whose tasks this
	// snapshot could not read: a failed load, or no tasks.md on disk. Their
	// tasks are unknown rather than absent, so nothing may be pruned on the
	// strength of not finding one here.
	UnavailableProjectIDs() []string
}

// Repository is the sole persistence boundary for user mutations in the
// application layer. Snapshot loads a revision-consistent read model; Commit
// applies one or more changesets atomically and reports each new revision.
// Concrete implementations wrap the transactional storage commit service; the
// contract tests inject an in-memory repository with the same semantics.
type Repository interface {
	// Snapshot loads a revision-consistent read model of the workspace.
	Snapshot() (Snapshot, error)
	// Commit applies the changesets atomically and returns one result per
	// changeset, in order. A stale expected revision aborts the whole commit.
	Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error)
}

// snapshotView is the concrete Snapshot the application layer builds over a set
// of loaded projects and their revisions. Its UUID index retains every owner so
// unqualified lookups can reject cross-project collisions.
type snapshotView struct {
	projects  []*models.Project
	byProject map[string]*models.Project
	byTask    map[string][]TaskRef
	byLocator map[TaskLocator]TaskRef
	revisions map[string]string
	documents map[string]*storage.ProjectDocument
	// unavailable is the sorted set UnavailableProjectIDs answers with.
	unavailable []string
}

// NewPartialSnapshot builds a Snapshot over the given projects, loaded
// revisions, and exact lossless documents. The document map must contain the
// parse retained at the same load as each project/revision; application
// mutations treat those documents as immutable merge bases. unavailable names
// the cataloged projects the read could not see, whose tasks are unknown; nil
// means it saw every one.
func NewPartialSnapshot(projects []*models.Project, revisions map[string]string, documents map[string]*storage.ProjectDocument, unavailable []string) Snapshot {
	// The task indexes are sized up front: a large workspace otherwise pays for
	// growing them a dozen times on every snapshot a mutation takes.
	taskCount := 0
	for _, project := range projects {
		if project != nil {
			taskCount += len(project.Tasks)
		}
	}
	view := &snapshotView{
		byProject: make(map[string]*models.Project, len(projects)),
		byTask:    make(map[string][]TaskRef, taskCount),
		byLocator: make(map[TaskLocator]TaskRef, taskCount),
		revisions: make(map[string]string, len(revisions)),
		documents: make(map[string]*storage.ProjectDocument, len(documents)),
	}
	for _, id := range unavailable {
		if id != "" && !slices.Contains(view.unavailable, id) {
			view.unavailable = append(view.unavailable, id)
		}
	}
	slices.Sort(view.unavailable)
	for id, rev := range revisions {
		view.revisions[id] = rev
	}
	for id, doc := range documents {
		view.documents[id] = doc
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		view.projects = append(view.projects, project)
		view.byProject[project.ID] = project
		for _, task := range project.Tasks {
			if task == nil || task.ID == "" {
				continue
			}
			ref := TaskRef{Task: task, Project: project, Base: view.documents[project.ID]}
			view.byLocator[TaskLocator{ProjectID: project.ID, TaskID: task.ID}] = ref
			view.byTask[task.ID] = append(view.byTask[task.ID], ref)
		}
	}
	return view
}

func (v *snapshotView) ProjectByID(projectID string) *models.Project {
	if projectID == "" {
		return nil
	}
	return v.byProject[projectID]
}

func (v *snapshotView) LookupTaskID(taskID string) (TaskRef, TaskLookupStatus) {
	if taskID == "" {
		return TaskRef{}, TaskLookupMissing
	}
	refs := v.byTask[taskID]
	switch len(refs) {
	case 0:
		return TaskRef{}, TaskLookupMissing
	case 1:
		return refs[0], TaskLookupUnique
	default:
		return TaskRef{}, TaskLookupAmbiguous
	}
}

func (v *snapshotView) LookupLocator(locator TaskLocator) (TaskRef, bool) {
	if !locator.Valid() {
		return TaskRef{}, false
	}
	ref, ok := v.byLocator[locator]
	return ref, ok
}

func (v *snapshotView) Revision(projectID string) string {
	return v.revisions[projectID]
}

func (v *snapshotView) Document(projectID string) *storage.ProjectDocument {
	return v.documents[projectID]
}

func (v *snapshotView) Projects() []*models.Project { return v.projects }

func (v *snapshotView) UnavailableProjectIDs() []string { return slices.Clone(v.unavailable) }
