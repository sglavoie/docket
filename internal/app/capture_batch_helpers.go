package app

import (
	"errors"
	"fmt"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// createdCaptureProjects indexes the projects a capture creates by identity,
// each a copy holding no task, after refusing one the snapshot already holds by
// identity or by slug: the creation was prepared against an earlier read, and a
// project minted since then is the race that read could not see.
func createdCaptureProjects(snap Snapshot, creation ProjectCreation) (map[string]*models.Project, error) {
	if len(creation.Projects) == 0 {
		return nil, nil
	}
	if creation.Catalog.Path == "" || creation.Catalog.ExpectedRevision == nil {
		return nil, fmt.Errorf("app: created projects carry no checked catalog artifact")
	}
	held := make(map[string]bool)
	for _, project := range snap.Projects() {
		if project != nil {
			held[project.Slug] = true
		}
	}
	created := make(map[string]*models.Project, len(creation.Projects))
	for _, project := range creation.Projects {
		if project == nil || project.ID == "" {
			return nil, invalidInput("project", "a new project needs an identity")
		}
		if snap.ProjectByID(project.ID) != nil || created[project.ID] != nil {
			return nil, invalidInput("project", fmt.Sprintf("project %s already exists", project.ID))
		}
		if held[project.Slug] {
			return nil, invalidInput("project name", fmt.Sprintf("project with slug '%s' already exists", project.Slug))
		}
		held[project.Slug] = true
		clone := *project
		clone.Tasks = nil
		created[project.ID] = &clone
	}
	return created, nil
}

// createdProjectFileError names the project whose tasks.md a capture found
// already on disk, keeping the storage conflict wrapped beneath it.
func createdProjectFileError(created map[string]*models.Project, err error) error {
	var conflict *storage.ArtifactConflictError
	if !errors.As(err, &conflict) || conflict.Expected != "" {
		return err
	}
	for _, project := range created {
		if conflict.Path == storage.TasksFilePath(project.Path) {
			return fmt.Errorf("project '%s' cannot be created: %s already exists on disk: %w",
				project.Slug, conflict.Path, err)
		}
	}
	return err
}

// captureProject resolves a request's project from the snapshot, else from
// the projects this capture creates.
func captureProject(snap Snapshot, created map[string]*models.Project, projectID string) *models.Project {
	if project := snap.ProjectByID(projectID); project != nil {
		return project
	}
	return created[projectID]
}

// requireCreatedProjectsReceiveTasks refuses a created project no request
// files into: the catalog would record it with no tasks.md written.
func requireCreatedProjectsReceiveTasks(creation ProjectCreation, filed map[string]bool) error {
	for _, project := range creation.Projects {
		if !filed[project.ID] {
			return invalidInput("project", fmt.Sprintf("new project %s receives no task", project.Slug))
		}
	}
	return nil
}

// withCatalog adds the creation's catalog to a commit's artifacts when it
// creates anything.
func (c ProjectCreation) withCatalog(artifacts []storage.ArtifactChange) []storage.ArtifactChange {
	if len(c.Projects) == 0 {
		return artifacts
	}
	return append(artifacts, c.Catalog)
}

func captureRequestError(req CaptureRequest, index int, err error) error {
	return fmt.Errorf("line %d: %w", captureRequestLine(req, index), err)
}

// captureRequestLine is the line number a batch failure is attributed to,
// falling back to the request's position when the request did not come from a
// multiline composer.
func captureRequestLine(req CaptureRequest, index int) int {
	if req.SourceLine > 0 {
		return req.SourceLine
	}
	return index + 1
}

// planningArtifactsForTargets renders one checked planning artifact set for a
// capture batch, dropping the memberships of any task the same transaction
// removes. Calling the builder once is important: every created task must be
// added to Today/Next in the same planning snapshot and the same ChangeSet, and
// a removal has to leave from that same snapshot.
func planningArtifactsForTargets(targets []PlanningReconcileTarget, remove []TaskLocator, builder PlanningArtifactBuilder) ([]storage.ArtifactChange, error) {
	if len(targets) == 0 && len(remove) == 0 {
		return nil, nil
	}
	if builder == nil {
		// Only an add refuses here. A removal without a builder falls through the
		// way Delete and BulkDelete already do: the membership is left to read-time
		// filtering rather than failing a deletion the user confirmed.
		for _, target := range targets {
			if target.Today == PlanningTodayAdd || target.Next == PlanningNextAdd {
				return nil, invalidInput("destination", "planning persistence is unavailable")
			}
		}
		return nil, nil
	}
	artifacts, err := builder(PlanningReconcileRequest{Targets: targets, Remove: remove})
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			return nil, fmt.Errorf("app: planning artifact %q has no expected revision", artifact.Path)
		}
	}
	return artifacts, nil
}

func validateCaptureBatch(reqs []CaptureRequest) error {
	for i, req := range reqs {
		if req.Submission == nil {
			continue
		}
		if i != 0 {
			return invalidInput("capture", "submission identity belongs to the first request only")
		}
		if err := req.Submission.Validate(); err != nil {
			return err
		}
	}

	if len(reqs) == 0 {
		return invalidInput("capture", "no tasks to create")
	}

	return nil
}

type captureBatchItem struct {
	project  *models.Project
	prepared preparedCapture
	task     *models.Task
}

func captureBatchReceipt(submission *storage.CaptureSubmission, items []captureBatchItem) *storage.CaptureReceipt {
	if submission == nil {
		return nil
	}
	titles := make([]string, len(items))
	for i, item := range items {
		titles[i] = item.task.Title
	}
	return &storage.CaptureReceipt{CaptureSubmission: *submission, Titles: titles}
}
