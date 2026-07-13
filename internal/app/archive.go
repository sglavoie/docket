package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/storage"
)

// ArchiveResult is the presentation-neutral outcome of archiving a project's
// completed tasks: the owning project's identity, how many tasks were archived,
// and the project's post-commit canonical revision. ArchivedCount is 0 when the
// project had no completed tasks, in which case nothing was committed.
type ArchiveResult struct {
	// ProjectID is the archived project's stable ID.
	ProjectID string
	// ProjectSlug is the archived project's slug.
	ProjectSlug string
	// ProjectName is the archived project's display name.
	ProjectName string
	// ArchivedCount is the number of completed tasks moved to the archive.
	ArchivedCount int
	// Revision is the project's new canonical revision after the commit, or "" when
	// nothing was archived.
	Revision string
}

// ArchiveTasks archives every completed task in the given project, committing the
// active-task removal (the project's tasks.md rewrite), the archive file
// append/rotation, the refreshed archive index, the archived tasks' attachment
// retention deletes, and any caller-supplied durable config artifacts as ONE
// atomic, revision-checked, ledger-backed transaction. A crash or failure leaves
// the project fully un-archived, and — because the commit rides the workspace undo
// ledger — the archive is undoable. Caller artifacts (typically the durable
// planning refs cleaned of the archived task IDs) ride the same transaction so a
// partial archive can never leave planning refs pointing at an archived task;
// machine-local recent visits are deliberately excluded and repaired separately by
// the adapter after a successful commit. When the project has no completed tasks
// nothing is committed and ArchivedCount is 0.
func (s *Service) ArchiveTasks(projectID string, artifacts ...storage.ArtifactChange) (ArchiveResult, error) {
	_, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return ArchiveResult{}, err
	}

	plan, err := storage.PlanArchiveTasks(project)
	if err != nil {
		return ArchiveResult{}, err
	}
	if plan.Count == 0 {
		return ArchiveResult{
			ProjectID:   project.ID,
			ProjectSlug: project.Slug,
			ProjectName: project.Name,
		}, nil
	}

	// The archive file rewrites and index are pre-computed bytes; the project write
	// carries the active-task removal. The caller's durable planning artifacts ride
	// the same changeset so the cleanup commits atomically with the archive.
	arts := make([]storage.ArtifactChange, 0, len(plan.Artifacts)+len(artifacts))
	arts = append(arts, plan.Artifacts...)
	arts = append(arts, artifacts...)

	applyPostMutationHooks(project)
	results, err := s.repo.Commit(storage.ChangeSet{
		Project:          project,
		ExpectedRevision: revision,
		Artifacts:        arts,
		AttachmentOps:    plan.AttachmentOps,
		UndoAction:       "Archive Tasks",
		UndoContext:      archiveTaskCountContext(plan.Count),
	})
	if err != nil {
		return ArchiveResult{}, err
	}

	newRevision := ""
	if len(results) > 0 && results[0] != nil {
		newRevision = results[0].Revision
	}
	return ArchiveResult{
		ProjectID:     project.ID,
		ProjectSlug:   project.Slug,
		ProjectName:   project.Name,
		ArchivedCount: plan.Count,
		Revision:      newRevision,
	}, nil
}

// archiveTaskCountContext renders the undo-context task count for an archive.
func archiveTaskCountContext(n int) string {
	if n == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", n)
}

// ArchiveAllResult reports a multi-project archive: one ArchiveResult per project
// that actually had completed tasks (projects with none are skipped), the total
// tasks archived, and every project committed in the single transaction.
type ArchiveAllResult struct {
	// Projects carries one result per archived project, in the order supplied.
	Projects []ArchiveResult
	// TotalArchived is the total number of tasks archived across all projects.
	TotalArchived int
	// TouchedProjectIDs lists every project committed together, in commit order.
	TouchedProjectIDs []string
}

// ArchiveAllTasks archives the completed tasks of several projects as ONE atomic,
// ledger-backed transaction: every project's active-task removal, archive file
// rewrites, refreshed index, and attachment retention commit together, so a single
// undo reverses the whole batch and a failure leaves every project un-archived.
// Projects with no completed tasks are skipped (they contribute no changeset).
// Caller-supplied durable config artifacts (typically the planning refs cleaned of
// every archived task) ride the first committed project's changeset so the cleanup
// commits atomically with the archive. When no project has completed tasks nothing
// is committed and TotalArchived is 0.
func (s *Service) ArchiveAllTasks(projectIDs []string, artifacts ...storage.ArtifactChange) (ArchiveAllResult, error) {
	if len(projectIDs) == 0 {
		return ArchiveAllResult{}, nil
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return ArchiveAllResult{}, err
	}

	var commits []projectCommit
	var planned []ArchiveResult
	total := 0
	for _, pid := range projectIDs {
		project := snap.ProjectByID(pid)
		if project == nil {
			return ArchiveAllResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, pid)
		}
		plan, err := storage.PlanArchiveTasks(project)
		if err != nil {
			return ArchiveAllResult{}, err
		}
		if plan.Count == 0 {
			continue
		}
		arts := plan.Artifacts
		if len(commits) == 0 && len(artifacts) > 0 {
			// Fold the caller's durable config artifacts onto the first committed
			// project so the cleanup commits atomically with the whole archive batch.
			arts = make([]storage.ArtifactChange, 0, len(plan.Artifacts)+len(artifacts))
			arts = append(arts, plan.Artifacts...)
			arts = append(arts, artifacts...)
		}
		commits = append(commits, projectCommit{
			project:          project,
			expectedRevision: snap.Revision(pid),
			artifacts:        arts,
			attachmentOps:    plan.AttachmentOps,
		})
		planned = append(planned, ArchiveResult{
			ProjectID:     project.ID,
			ProjectSlug:   project.Slug,
			ProjectName:   project.Name,
			ArchivedCount: plan.Count,
		})
		total += plan.Count
	}
	if len(commits) == 0 {
		return ArchiveAllResult{}, nil
	}

	results, err := s.commitProjects(commits, "Archive Tasks", fmt.Sprintf("%d tasks across %d projects", total, len(commits)))
	if err != nil {
		return ArchiveAllResult{}, err
	}

	out := ArchiveAllResult{Projects: make([]ArchiveResult, 0, len(planned)), TotalArchived: total}
	for i := range planned {
		planned[i].Revision = revisionAt(results, i)
		out.Projects = append(out.Projects, planned[i])
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, planned[i].ProjectID)
	}
	return out, nil
}
