package app

import (
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ReferenceDocument pairs one catalog project with everything a soft-reference
// rewrite needs to commit it: the loaded project, the exact lossless document it
// was parsed from, the canonical revision a ChangeSet compares, the raw
// tasks.md byte hash an ArtifactChange compares, and that file's path.
//
// Both revisions are carried because the two commit boundaries ask different
// questions of the same file: a project ChangeSet validates the canonical
// document revision, while an ops transaction writing tasks.md as an artifact
// validates the raw bytes. A pass that had only one of them would have to guess
// at the other.
type ReferenceDocument struct {
	Project     *models.Project
	Base        *storage.ProjectDocument
	Revision    string
	RawRevision string
	TasksPath   string
}

// ReferenceDocumentSource is the optional Repository capability that enumerates
// every catalog project document, archived ones included. It is deliberately
// separate from Snapshot: the mutation read model stays the population a user
// can act on, so widening reference maintenance to the whole catalog never
// widens the candidate set a move, a capture, or a picker can name.
//
// A repository that does not implement it (an in-memory double) simply has no
// documents beyond its snapshot, which is exactly what the interface then
// reports.
type ReferenceDocumentSource interface {
	// ReferenceDocuments returns one entry per catalog project, loaded at a single
	// point in time. A project whose document cannot be read is an error, never a
	// silent omission: a document nobody could inspect is a document whose stale
	// locators nobody could repair.
	ReferenceDocuments() ([]ReferenceDocument, error)
}

// TaskReferenceMaintenance is the relocation-and-removal delta every
// locator-changing or destructive mutation hands to the maintenance pass. It
// describes what happened to reference *targets*; the pass turns that into the
// rewrites their holders need.
//
// The three axes are kept apart because they are different facts: a moved task
// still exists at a new address, a removed task exists nowhere, and a removed
// project takes every address under it. Nothing here describes archiving or
// renaming, which change no identity and therefore need no rewrite at all.
type TaskReferenceMaintenance struct {
	// Moved maps a task's pre-mutation locator to the one it now owns. Only the
	// project component can differ: task identities are workspace-stable.
	Moved map[models.TaskReference]models.TaskReference
	// RemovedTasks are locators whose task no longer exists anywhere.
	RemovedTasks map[models.TaskReference]bool
	// RemovedProjects are project identities whose whole document is going away.
	RemovedProjects map[string]bool
}

// IsZero reports whether the delta describes nothing, which is what makes the
// whole pass a no-op for a same-project move: an internal reorder, reparent,
// indent, or outdent changes no locator, so there is nothing to rewrite.
func (m TaskReferenceMaintenance) IsZero() bool {
	return len(m.Moved) == 0 && len(m.RemovedTasks) == 0 && len(m.RemovedProjects) == 0
}

// rewriteReferences produces the reference list one task should hold after the
// delta. owner is the task's own post-mutation locator, which is what lets a
// reference that followed its holder to a single address collapse instead of
// becoming a self-reference the domain forbids.
//
// A holder's outgoing references are otherwise left exactly as they were: only a
// target named by the delta is touched, so moving a task never rewrites the
// references it carries with it.
func (m TaskReferenceMaintenance) rewriteReferences(owner models.TaskReference, existing []models.TaskReference) ([]models.TaskReference, bool) {
	if len(existing) == 0 {
		return existing, false
	}
	next := make([]models.TaskReference, 0, len(existing))
	for _, reference := range existing {
		if m.RemovedProjects[reference.ProjectID] || m.RemovedTasks[reference] {
			continue
		}
		if moved, ok := m.Moved[reference]; ok {
			reference = moved
		}
		if reference == owner {
			continue
		}
		if slices.Contains(next, reference) {
			continue
		}
		next = append(next, reference)
	}
	if models.TaskReferencesEqual(existing, next) {
		return existing, false
	}
	return models.CloneTaskReferences(next), true
}

// applyToProject rewrites every task in one project and reports whether the
// document changed. It deliberately leaves UpdatedAt alone: maintenance
// preserves a reference's meaning across someone else's move, and a task whose
// stored locator was re-addressed was not itself touched by the user.
func (m TaskReferenceMaintenance) applyToProject(project *models.Project) bool {
	if project == nil || m.IsZero() {
		return false
	}
	changed := false
	for _, task := range project.Tasks {
		if task == nil || task.ID == "" || len(task.Dependencies) == 0 {
			continue
		}
		owner := models.TaskReference{ProjectID: project.ID, TaskID: task.ID}
		next, taskChanged := m.rewriteReferences(owner, task.Dependencies)
		if !taskChanged {
			continue
		}
		task.Dependencies = next
		changed = true
	}
	return changed
}

// referenceMaintenancePass is the outcome of one maintenance run: the extra
// project commits the caller's transaction must carry, and the restore that puts
// the rewritten in-memory projects back when that transaction does not land.
type referenceMaintenancePass struct {
	commits []projectCommit
	backups []hierarchyProjectBackup
}

// restore reverses every rewrite this pass performed, newest first, so a
// refused or conflicted commit leaves no partially repaired workspace behind.
func (p referenceMaintenancePass) restore() {
	for i := len(p.backups) - 1; i >= 0; i-- {
		p.backups[i].restore()
	}
}

// maintainTaskReferences applies one delta across the whole catalog and returns
// the additional project commits the mutation must include. committing names the
// projects the caller already commits itself; those are rewritten in place and
// must not gain a second changeset for the same document.
//
// Snapshot projects are rewritten through the caller's own revision-consistent
// read model. Every other catalog project — archived ones above all — comes from
// the reference-document source with its own revision, so a stale document
// aborts the whole commit rather than half-repairing the workspace.
func (s *Service) maintainTaskReferences(snap Snapshot, maintenance TaskReferenceMaintenance, committing map[string]bool) (referenceMaintenancePass, error) {
	var pass referenceMaintenancePass
	if maintenance.IsZero() || snap == nil {
		return pass, nil
	}
	loaded := make(map[string]bool)
	for _, project := range snap.Projects() {
		if project == nil {
			continue
		}
		loaded[project.ID] = true
		backup := captureHierarchyProject(project)
		if !maintenance.applyToProject(project) {
			continue
		}
		pass.backups = append(pass.backups, backup)
		if committing[project.ID] {
			continue
		}
		pass.commits = append(pass.commits, projectCommit{
			project:          project,
			expectedRevision: snap.Revision(project.ID),
			base:             snap.Document(project.ID),
			tasksUntouched:   true,
		})
	}

	documents, err := s.referenceDocuments()
	if err != nil {
		pass.restore()
		return referenceMaintenancePass{}, err
	}
	for _, document := range documents {
		if document.Project == nil || loaded[document.Project.ID] {
			continue
		}
		backup := captureHierarchyProject(document.Project)
		if !maintenance.applyToProject(document.Project) {
			continue
		}
		pass.backups = append(pass.backups, backup)
		pass.commits = append(pass.commits, projectCommit{
			project:          document.Project,
			expectedRevision: document.Revision,
			base:             document.Base,
			tasksUntouched:   true,
		})
	}
	return pass, nil
}

// referenceDocuments enumerates the catalog documents beyond the mutation
// snapshot. A repository without the capability reports none, which leaves the
// snapshot itself as the whole reference scope.
func (s *Service) referenceDocuments() ([]ReferenceDocument, error) {
	source, ok := s.repo.(ReferenceDocumentSource)
	if !ok {
		return nil, nil
	}
	return source.ReferenceDocuments()
}

// referenceArtifactsForRemovedProject renders the tasks.md rewrite of every
// catalog project that references the project being deleted. Project deletion
// commits raw filesystem choreography rather than project changesets, so each
// rewrite rides as a revision-checked artifact — the same shape a rename uses
// for the renamed project's own tasks.md.
func (s *Service) referenceArtifactsForRemovedProject(projectID string) ([]storage.ArtifactChange, error) {
	if projectID == "" {
		return nil, nil
	}
	documents, err := s.referenceDocuments()
	if err != nil {
		return nil, err
	}
	maintenance := TaskReferenceMaintenance{RemovedProjects: map[string]bool{projectID: true}}
	var artifacts []storage.ArtifactChange
	for _, document := range documents {
		if document.Project == nil || document.Project.ID == projectID || document.TasksPath == "" {
			continue
		}
		backup := captureHierarchyProject(document.Project)
		if !maintenance.applyToProject(document.Project) {
			continue
		}
		// The rewrite exists to produce bytes, so the in-memory project is put back
		// immediately: this path commits raw operations, and a repository that hands
		// out live pointers must not observe a rewrite whose transaction may still be
		// refused.
		defer backup.restore()
		canonical, renderErr := storage.CanonicalProjectDocument(document.Project, document.Base)
		if renderErr != nil {
			return nil, fmt.Errorf("rewrite references in project %s: %w", document.Project.Name, renderErr)
		}
		content, writeErr := canonical.WriteCurrent()
		if writeErr != nil {
			return nil, fmt.Errorf("rewrite references in project %s: %w", document.Project.Name, writeErr)
		}
		expected := document.RawRevision
		artifacts = append(artifacts, storage.ArtifactChange{
			Path:             document.TasksPath,
			Data:             []byte(content),
			Mode:             storage.FilePermissions,
			ExpectedRevision: &expected,
		})
	}
	return artifacts, nil
}
