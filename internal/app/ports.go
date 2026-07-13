// Package app is the presentation-neutral application service layer for tt. It
// exposes narrow task-mutation use cases (capture, edit, state transition, due
// date, priority, tag, pin, snooze, park, completion) that validate input,
// apply domain rules through the models package, and emit exactly one storage
// ChangeSet committed through the repository boundary. It imports neither Cobra
// nor Bubble Tea: the CLI and TUI adapters (later sessions) drive these use
// cases and translate the presentation-neutral results and typed errors into
// their own output. Every dependency the layer needs — the mutation repository,
// a clock, and an ID generator — is an injected interface so the use cases are
// deterministic and testable against an in-memory repository.
package app

import (
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
	"github.com/sglavoie/tt/internal/workspace"
)

// OpsCommitter is the optional capability a Repository exposes to commit a set of
// raw file operations — a directory move, a tombstone write, a config rewrite, a
// recursive delete — as ONE atomic journaled transaction. It is the persistence
// boundary the project archive-location, trash, restore, and purge use cases drive:
// where task mutations flow through Commit(ChangeSet), the config- and directory-
// centric project lifecycle flows through CommitOps. The storage-backed CLI
// repository implements it over CommitService.CommitOps; a repository that does not
// support it makes the project-lifecycle use cases fail closed.
type OpsCommitter interface {
	CommitOps(ops []transaction.Op) error
}

// Clock supplies wall-clock time to the use cases. Injecting it keeps
// time-derived behavior (snooze targets, due dates) deterministic under test.
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
}

// Snapshot is the revision-consistent read model the application layer consumes
// before a mutation: it resolves task and project references and reports the
// canonical revision each project was loaded at, which the use cases capture as
// the optimistic-concurrency expectation on the emitted ChangeSet.
type Snapshot interface {
	// ProjectByID returns the loaded project with the given ID, or nil.
	ProjectByID(projectID string) *models.Project
	// Lookup resolves a task by its UUID to the task and its owning project.
	Lookup(taskID string) (TaskRef, bool)
	// Revision returns the canonical content revision a project was loaded at,
	// or "" when the project is not part of the snapshot.
	Revision(projectID string) string
	// Projects returns every loaded project.
	Projects() []*models.Project
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
// of loaded projects and their revisions. It indexes tasks by UUID for O(1)
// resolution; a later duplicate task ID does not overwrite the first-seen
// location, mirroring the fail-closed cross-project resolution used elsewhere.
type snapshotView struct {
	projects  []*models.Project
	byProject map[string]*models.Project
	byTask    map[string]TaskRef
	revisions map[string]string
}

// NewSnapshot builds a Snapshot over the given projects and their loaded
// revisions. It is the seam both the in-memory repository (tests) and the
// storage-backed repository (later sessions) use to hand the use cases a read
// model without exposing their internal loading.
func NewSnapshot(projects []*models.Project, revisions map[string]string) Snapshot {
	view := &snapshotView{
		byProject: make(map[string]*models.Project, len(projects)),
		byTask:    make(map[string]TaskRef),
		revisions: make(map[string]string, len(revisions)),
	}
	for id, rev := range revisions {
		view.revisions[id] = rev
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
			if _, seen := view.byTask[task.ID]; !seen {
				view.byTask[task.ID] = TaskRef{Task: task, Project: project}
			}
		}
	}
	return view
}

// SnapshotFromWorkspace adapts an already-built workspace read model into the
// application layer's Snapshot, reusing its per-project revisions instead of
// recomputing them. It is how a storage-backed repository turns a
// workspace.Snapshot into the read model the use cases consume.
func SnapshotFromWorkspace(ws *workspace.Snapshot) Snapshot {
	if ws == nil {
		return NewSnapshot(nil, nil)
	}
	projects := ws.Projects()
	revisions := make(map[string]string, len(projects))
	for _, project := range projects {
		revisions[project.ID] = ws.Revision(project.ID)
	}
	return NewSnapshot(projects, revisions)
}

func (v *snapshotView) ProjectByID(projectID string) *models.Project {
	if projectID == "" {
		return nil
	}
	return v.byProject[projectID]
}

func (v *snapshotView) Lookup(taskID string) (TaskRef, bool) {
	if taskID == "" {
		return TaskRef{}, false
	}
	ref, ok := v.byTask[taskID]
	return ref, ok
}

func (v *snapshotView) Revision(projectID string) string {
	return v.revisions[projectID]
}

func (v *snapshotView) Projects() []*models.Project { return v.projects }
