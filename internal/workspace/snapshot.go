// Package workspace provides an immutable read model over the loaded projects
// of a Docket workspace. A Snapshot loads every requested project exactly once and
// exposes derived indexes (task lookup, qualified references, per-project
// revisions) plus diagnostics for projects that could
// not be loaded. Read-only consumers — workspace pulse, cross-scope task views,
// global search, task resolution — build a snapshot instead of independently
// re-reading and re-parsing tasks.md, so a single revision-consistent view
// backs the whole render pass.
package workspace

import (
	"fmt"
	"sync"

	"github.com/sglavoie/docket/internal/models"
)

// DiagnosticKind classifies why a project failed to contribute complete data to
// a snapshot. Surfacing the kind lets callers report incomplete results instead
// of silently returning a partial task set.
type DiagnosticKind string

const (
	// DiagnosticCorrupt marks a project whose tasks.md could not be parsed.
	DiagnosticCorrupt DiagnosticKind = "corrupt"
	// DiagnosticSkipped marks a project that failed to load for any other
	// reason (e.g. an unreadable file) and was excluded from the snapshot.
	DiagnosticSkipped DiagnosticKind = "skipped"
)

// Diagnostic records a project that was skipped or partially loaded while
// building a snapshot.
type Diagnostic struct {
	ProjectID   string
	ProjectName string
	Kind        DiagnosticKind
	Detail      string
}

// String renders a human-readable one-line diagnostic suitable for logs or a
// status readout.
func (d Diagnostic) String() string {
	name := d.ProjectName
	if name == "" {
		name = d.ProjectID
	}
	if name == "" {
		name = "<unknown>"
	}
	return fmt.Sprintf("%s project %s: %s", d.Kind, name, d.Detail)
}

// TaskLocation pairs a task with the project that owns it. It is the value type
// returned by the snapshot's task-lookup index.
type TaskLocation struct {
	Task    *models.Task
	Project *models.Project
}

// Snapshot is an immutable read model over a set of loaded projects. All fields
// are populated at build time and must not be mutated afterward; consumers that
// need to mutate a project must load their own copy through the normal save
// path rather than editing snapshot state.
//
// The task indexes are derived on first use rather than at build time. Most
// snapshots are read for their projects alone — the Outliner's reload and the
// workspace pulse never look a task up — and indexing every task of a large
// workspace was most of what building one cost.
type Snapshot struct {
	projects    []*models.Project
	revisions   map[string]string
	diagnostics []Diagnostic

	index sync.Once
	tasks []*models.Task
	byID  map[string][]TaskLocation
}

func newSnapshot() *Snapshot {
	return &Snapshot{revisions: make(map[string]string)}
}

// addProject folds one successfully loaded project (and its computed revision)
// into the snapshot.
func (s *Snapshot) addProject(project *models.Project, revision string) {
	s.projects = append(s.projects, project)
	s.revisions[project.ID] = revision
}

// buildIndex derives the flat task union and the UUID index. Every task owner is
// retained so an unqualified duplicate UUID can be reported as ambiguous instead
// of selecting whichever project happened to load first.
func (s *Snapshot) buildIndex() {
	s.index.Do(func() {
		count := 0
		for _, project := range s.projects {
			count += len(project.Tasks)
		}
		s.tasks = make([]*models.Task, 0, count)
		s.byID = make(map[string][]TaskLocation, count)
		for _, project := range s.projects {
			for _, task := range project.Tasks {
				if task == nil {
					continue
				}
				s.tasks = append(s.tasks, task)
				if task.ID != "" {
					s.byID[task.ID] = append(s.byID[task.ID], TaskLocation{Task: task, Project: project})
				}
			}
		}
	})
}

func (s *Snapshot) addDiagnostic(diag Diagnostic) {
	s.diagnostics = append(s.diagnostics, diag)
}

// Projects returns the successfully loaded projects, in the order requested.
func (s *Snapshot) Projects() []*models.Project { return s.projects }

// Tasks returns the flat union of every task across the loaded projects.
func (s *Snapshot) Tasks() []*models.Task {
	s.buildIndex()
	return s.tasks
}

// TaskLookupStatus describes whether an unqualified UUID is missing, uniquely
// owned, or present in more than one loaded project.
type TaskLookupStatus uint8

const (
	TaskLookupMissing TaskLookupStatus = iota
	TaskLookupUnique
	TaskLookupAmbiguous
)

// LookupTaskID resolves a task by UUID only when the workspace has one owner.
func (s *Snapshot) LookupTaskID(taskID string) (TaskLocation, TaskLookupStatus) {
	if taskID == "" {
		return TaskLocation{}, TaskLookupMissing
	}
	s.buildIndex()
	locations := s.byID[taskID]
	switch len(locations) {
	case 0:
		return TaskLocation{}, TaskLookupMissing
	case 1:
		return locations[0], TaskLookupUnique
	default:
		return TaskLocation{}, TaskLookupAmbiguous
	}
}

// Revision returns the canonical content revision recorded for a project at
// load time, or "" when the project is not part of the snapshot.
func (s *Snapshot) Revision(projectID string) string {
	return s.revisions[projectID]
}

// QualifiedRef returns the canonical "slug:#short-id" reference for a task, or
// "" when the task is not part of the snapshot. Cross-project surfaces use this
// so their output always carries an unambiguous, project-qualified reference.
func (s *Snapshot) QualifiedRef(taskID string) string {
	location, status := s.LookupTaskID(taskID)
	if status != TaskLookupUnique {
		return ""
	}
	return fmt.Sprintf("%s:#%d", location.Project.Slug, location.Task.ShortID)
}

// Diagnostics returns the projects that were skipped or failed to parse while
// building the snapshot.
func (s *Snapshot) Diagnostics() []Diagnostic { return s.diagnostics }

// HasDiagnostics reports whether any project was skipped or failed to load.
func (s *Snapshot) HasDiagnostics() bool { return len(s.diagnostics) > 0 }
