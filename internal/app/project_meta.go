package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectTagResult is the presentation-neutral outcome of a project pinned-tag
// mutation.
type ProjectTagResult struct {
	// ProjectID is the affected project's stable ID.
	ProjectID string
	// ProjectName is the affected project's display name.
	ProjectName string
	// Tag is the normalized tag the mutation targeted.
	Tag string
	// Pinned is the tag's pinned state after the mutation.
	Pinned bool
	// Changed reports whether the mutation actually changed anything (pinning an
	// already-pinned tag or unpinning an unpinned one is a no-op).
	Changed bool
	// Revision is the project's new canonical revision after the commit.
	Revision string
}

// PinProjectTag pins a tag as a quick-access tab on a project and commits the
// project as one ChangeSet. The tag is normalized and validated before any
// mutation; pinning an already-pinned tag is an idempotent no-op that still
// commits the (unchanged) project.
func (s *Service) PinProjectTag(projectID, tag string) (ProjectTagResult, error) {
	normalized := models.NormalizeTag(tag)
	if err := models.ValidateTag(normalized); err != nil {
		return ProjectTagResult{}, invalidInput("tag", err.Error())
	}
	snap, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectTagResult{}, err
	}
	changed, err := project.PinTag(normalized)
	if err != nil {
		return ProjectTagResult{}, invalidInput("tag", err.Error())
	}
	rev, err := s.commitProjectMetadata(project, revision, snap.Document(project.ID), "Pin Tag", normalized)
	if err != nil {
		return ProjectTagResult{}, err
	}
	return ProjectTagResult{
		ProjectID:   project.ID,
		ProjectName: project.Name,
		Tag:         normalized,
		Pinned:      true,
		Changed:     changed,
		Revision:    rev,
	}, nil
}

// UnpinProjectTag removes a tag's quick-access pin from a project and commits
// the project as one ChangeSet. Unpinning a tag that is not pinned is an
// idempotent no-op that still commits the (unchanged) project.
func (s *Service) UnpinProjectTag(projectID, tag string) (ProjectTagResult, error) {
	normalized := models.NormalizeTag(tag)
	if err := models.ValidateTag(normalized); err != nil {
		return ProjectTagResult{}, invalidInput("tag", err.Error())
	}
	snap, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectTagResult{}, err
	}
	changed := project.UnpinTag(normalized)
	rev, err := s.commitProjectMetadata(project, revision, snap.Document(project.ID), "Unpin Tag", normalized)
	if err != nil {
		return ProjectTagResult{}, err
	}
	return ProjectTagResult{
		ProjectID:   project.ID,
		ProjectName: project.Name,
		Tag:         normalized,
		Pinned:      false,
		Changed:     changed,
		Revision:    rev,
	}, nil
}

// TagCatalogResult is the presentation-neutral outcome of a project tag-catalog
// mutation (add, remove, rename, pin toggle). The catalog is project metadata,
// but a rename also rewrites every task carrying the tag, so all catalog
// mutations commit the whole project as one ChangeSet.
type TagCatalogResult struct {
	// ProjectID is the affected project's stable ID.
	ProjectID string
	// Tag is the normalized tag the mutation targeted (for rename, the new name).
	Tag string
	// OldTag is the previous name (meaningful for RenameProjectTag only).
	OldTag string
	// Pinned is the tag's pinned state after the mutation (meaningful for
	// TogglePinnedProjectTag only).
	Pinned bool
	// Changed reports whether the mutation actually changed anything. An
	// unchanged mutation is not committed, so it records no undo entry.
	Changed bool
	// Revision is the project's new canonical revision ("" when unchanged).
	Revision string
}

// AddProjectTag adds a tag to the project catalog and commits the project as
// one ChangeSet. Adding a tag that already exists is rejected before any
// mutation so the adapter can surface the duplicate.
func (s *Service) AddProjectTag(projectID, tag string) (TagCatalogResult, error) {
	normalized := models.NormalizeTag(tag)
	snap, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return TagCatalogResult{}, err
	}
	added, err := project.AddTag(normalized)
	if err != nil {
		return TagCatalogResult{}, invalidInput("tag", err.Error())
	}
	if !added {
		return TagCatalogResult{}, invalidInput("tag", fmt.Sprintf("tag '%s' already exists", normalized))
	}
	rev, err := s.commitProjectMetadata(project, revision, snap.Document(project.ID), "Add Project Tag", normalized)
	if err != nil {
		return TagCatalogResult{}, err
	}
	return TagCatalogResult{ProjectID: project.ID, Tag: normalized, Changed: true, Revision: rev}, nil
}

// RemoveProjectTag deletes a tag from the project catalog, every task carrying
// it, and the pinned set, committing the project as one ChangeSet. Removing a
// tag that does not exist anywhere is rejected before any mutation.
func (s *Service) RemoveProjectTag(projectID, tag string) (TagCatalogResult, error) {
	normalized := models.NormalizeTag(tag)
	snap, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return TagCatalogResult{}, err
	}
	result, err := project.DeleteTag(normalized)
	if err != nil {
		return TagCatalogResult{}, invalidInput("tag", err.Error())
	}
	if !result.Changed() {
		return TagCatalogResult{}, invalidInput("tag", fmt.Sprintf("tag '%s' does not exist", normalized))
	}
	rev, err := s.commitProjectMetadata(project, revision, snap.Document(project.ID), "Remove Project Tag", result.Name)
	if err != nil {
		return TagCatalogResult{}, err
	}
	return TagCatalogResult{ProjectID: project.ID, Tag: result.Name, Changed: true, Revision: rev}, nil
}

// RenameProjectTag renames a tag across the project catalog, every task
// carrying it, and the pinned set, committing the project as one ChangeSet. A
// rename that changes nothing (unknown tag with no usages) returns
// Changed=false without committing, so no spurious undo entry is recorded.
func (s *Service) RenameProjectTag(projectID, oldName, newName string) (TagCatalogResult, error) {
	normalizedNew := models.NormalizeTag(newName)
	if normalizedNew == "" {
		return TagCatalogResult{}, invalidInput("tag", "new tag name is required")
	}
	snap, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return TagCatalogResult{}, err
	}
	changed, err := project.RenameTag(oldName, normalizedNew)
	if err != nil {
		return TagCatalogResult{}, invalidInput("tag", err.Error())
	}
	if !changed {
		return TagCatalogResult{ProjectID: project.ID, Tag: normalizedNew, OldTag: oldName}, nil
	}
	rev, err := s.commitProjectMetadata(project, revision, snap.Document(project.ID), "Rename Tag", fmt.Sprintf("%s -> %s", oldName, normalizedNew))
	if err != nil {
		return TagCatalogResult{}, err
	}
	return TagCatalogResult{ProjectID: project.ID, Tag: normalizedNew, OldTag: oldName, Changed: true, Revision: rev}, nil
}

// WorkspaceTagRenameResult is the presentation-neutral outcome of a
// workspace-wide tag rename. It reports how many projects and task usages the
// rename touched so the adapter can render the same "renamed in N projects / M
// tasks" feedback the legacy global tag manager showed.
type WorkspaceTagRenameResult struct {
	// OldTag is the normalized tag that was renamed.
	OldTag string
	// NewTag is the normalized replacement name.
	NewTag string
	// ProjectCount is the number of projects whose catalog, pins, or task usages
	// actually changed.
	ProjectCount int
	// TaskCount is the number of task usages rewritten across every touched
	// project.
	TaskCount int
	// TouchedProjectIDs lists every project committed by the rename, in the order
	// the snapshot enumerated them.
	TouchedProjectIDs []string
}

// RenameWorkspaceTag renames a normalized tag across every project that uses it —
// catalog, pinned set, and every task carrying it — committing every affected
// project as ONE transaction, so the rename is atomic across the workspace and
// records a single grouped undo/redo entry. Projects that do not carry the tag
// are left untouched; a rename that changes nothing anywhere returns
// ProjectCount 0 without committing (recording no spurious undo entry). It is the
// workspace-wide counterpart of RenameProjectTag, which renames within one
// project.
func (s *Service) RenameWorkspaceTag(oldName, newName string) (WorkspaceTagRenameResult, error) {
	normalizedOld := models.NormalizeTag(oldName)
	normalizedNew := models.NormalizeTag(newName)
	if normalizedNew == "" {
		return WorkspaceTagRenameResult{}, invalidInput("tag", "new tag name is required")
	}
	if err := models.ValidateTag(normalizedNew); err != nil {
		return WorkspaceTagRenameResult{}, invalidInput("tag", err.Error())
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return WorkspaceTagRenameResult{}, err
	}

	result := WorkspaceTagRenameResult{OldTag: normalizedOld, NewTag: normalizedNew}
	var commits []projectCommit
	for _, project := range snap.Projects() {
		if project == nil {
			continue
		}
		if !project.HasTag(normalizedOld) && !project.IsTagPinned(normalizedOld) {
			continue
		}
		// Count task usages before the rename, while the old tag is still present.
		usages := countTaskTagUsage(project, normalizedOld)
		modified, rerr := project.RenameTag(normalizedOld, normalizedNew)
		if rerr != nil {
			return WorkspaceTagRenameResult{}, invalidInput("tag", rerr.Error())
		}
		if !modified {
			continue
		}
		commits = append(commits, projectCommit{project: project, expectedRevision: snap.Revision(project.ID), base: snap.Document(project.ID)})
		result.ProjectCount++
		result.TaskCount += usages
		result.TouchedProjectIDs = append(result.TouchedProjectIDs, project.ID)
	}

	if len(commits) == 0 {
		return result, nil
	}

	context := fmt.Sprintf("%s -> %s", normalizedOld, normalizedNew)
	if _, err := s.commitProjects(commits, "Rename Global Tag", context); err != nil {
		return WorkspaceTagRenameResult{}, err
	}
	return result, nil
}

// countTaskTagUsage counts how many of a project's tasks carry the given tag.
func countTaskTagUsage(project *models.Project, tag string) int {
	count := 0
	for _, task := range project.Tasks {
		if task.HasTag(tag) {
			count++
		}
	}
	return count
}

// TogglePinnedProjectTag flips a tag's quick-access pin on the project and
// commits the project as one ChangeSet, reporting the post-toggle state.
func (s *Service) TogglePinnedProjectTag(projectID, tag string) (TagCatalogResult, error) {
	normalized := models.NormalizeTag(tag)
	snap, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return TagCatalogResult{}, err
	}
	pinned, err := project.TogglePinnedTag(normalized)
	if err != nil {
		return TagCatalogResult{}, invalidInput("tag", err.Error())
	}
	action := "Unpin Tag"
	if pinned {
		action = "Pin Tag"
	}
	rev, err := s.commitProjectMetadata(project, revision, snap.Document(project.ID), action, normalized)
	if err != nil {
		return TagCatalogResult{}, err
	}
	return TagCatalogResult{ProjectID: project.ID, Tag: normalized, Pinned: pinned, Changed: true, Revision: rev}, nil
}

// commitProjectMetadata commits a project-scoped (non-task) mutation — pinned
// tags and tag-catalog edits — as one ChangeSet, returning the project's new
// revision. It is the project-metadata counterpart of commitProject, which
// serves single-task mutations. Sandbox reset has its own project-plus-note
// transaction in sandbox.go.
func (s *Service) commitProjectMetadata(project *models.Project, expectedRevision string, base *storage.ProjectDocument, undoAction, undoContext string) (string, error) {
	applyPostMutationHooks(project)
	results, err := s.repo.Commit(storage.ChangeSet{
		Project:          project,
		ExpectedRevision: expectedRevision,
		Base:             base,
		UndoAction:       undoAction,
		UndoContext:      undoContext,
	})
	if err != nil {
		return "", err
	}
	return revisionAt(results, 0), nil
}
