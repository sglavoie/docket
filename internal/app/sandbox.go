package app

import "github.com/sglavoie/tt/internal/models"

// SandboxResetResult reports the outcome of clearing the sandbox project: the
// project cleared, how many tasks it held before the reset, and the new revision
// committed for the emptied project.
type SandboxResetResult struct {
	ProjectID    string
	ClearedTasks int
	Revision     string
}

// ResetSandbox empties the sandbox project — clearing its tasks, sections, tags,
// pinned tags, and short-ID counter back to defaults — and commits the emptied
// project as ONE undoable transaction, so a reset restores through the workspace
// undo ledger like any other user mutation. Non-task artifacts (notes) are not
// part of the project document; the caller clears them as a filesystem side
// effect, matching the pre-transaction behavior where undo captured only tasks.
func (s *Service) ResetSandbox(projectID string) (SandboxResetResult, error) {
	snap, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return SandboxResetResult{}, err
	}

	cleared := len(project.Tasks)
	project.Tasks = nil
	project.NextShortID = 1
	project.Sections = []string{models.SectionNext}
	project.SectionColors = make(map[string]string)
	project.Tags = []string{}
	project.PinnedTags = []string{}

	commits := []projectCommit{{
		project:          project,
		expectedRevision: revision,
		base:             snap.Document(project.ID),
	}}
	results, err := s.commitProjects(commits, "Reset Sandbox", project.Name)
	if err != nil {
		return SandboxResetResult{}, err
	}
	return SandboxResetResult{
		ProjectID:    project.ID,
		ClearedTasks: cleared,
		Revision:     revisionAt(results, 0),
	}, nil
}
