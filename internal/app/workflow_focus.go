package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// WorkflowFocusChange is the application-layer form of a workflow engine
// set_active side effect. ProjectID records the task's final owning project so
// Focus Queue additions remain correct when the same cascade also moves the
// task across projects.
type WorkflowFocusChange struct {
	TaskID    string
	ProjectID string
	Add       bool
}

// WorkflowFocusArtifactBuilder applies requested Focus Queue changes to the
// planning domain and renders the resulting revision-checked artifact. The
// application service attaches those artifacts to the same ChangeSet as the
// workflow's task mutations, making both effects atomic and grouped for undo.
type WorkflowFocusArtifactBuilder func([]WorkflowFocusChange) ([]storage.ArtifactChange, error)

func appendWorkflowFocusChanges(dst []WorkflowFocusChange, projectID string, changes []workflow.FocusChange) []WorkflowFocusChange {
	for _, change := range changes {
		dst = append(dst, WorkflowFocusChange{
			TaskID:    change.TaskID,
			ProjectID: projectID,
			Add:       change.Add,
		})
	}
	return dst
}

// resolveWorkflowFocusChanges de-duplicates requests by task (last request
// wins across independently evaluated source projects) and resolves the task's
// final owner after every deferred workflow move has been applied.
func resolveWorkflowFocusChanges(changes []WorkflowFocusChange, commits []projectCommit) []WorkflowFocusChange {
	if len(changes) == 0 {
		return nil
	}

	resolved := make([]WorkflowFocusChange, 0, len(changes))
	byTask := make(map[string]int, len(changes))
	for _, change := range changes {
		if change.TaskID == "" {
			continue
		}
		if idx, ok := byTask[change.TaskID]; ok {
			resolved[idx] = change
			continue
		}
		byTask[change.TaskID] = len(resolved)
		resolved = append(resolved, change)
	}

	owners := make(map[string]string)
	for _, commit := range commits {
		if commit.project == nil {
			continue
		}
		for _, task := range commit.project.Tasks {
			if task != nil && task.ID != "" {
				owners[task.ID] = commit.project.ID
			}
		}
	}
	for i := range resolved {
		if owner := owners[resolved[i].TaskID]; owner != "" {
			resolved[i].ProjectID = owner
		}
	}
	return resolved
}

// attachWorkflowFocusArtifacts builds and attaches the planning artifact to the
// first project commit. A workflow-generated Focus Queue side effect is never
// allowed to fall back to an unchecked or post-commit config write.
func attachWorkflowFocusArtifacts(
	commits []projectCommit,
	changes []WorkflowFocusChange,
	builder WorkflowFocusArtifactBuilder,
) ([]WorkflowFocusChange, error) {
	resolved := resolveWorkflowFocusChanges(changes, commits)
	if len(resolved) == 0 {
		return nil, nil
	}
	if len(commits) == 0 || builder == nil {
		return nil, fmt.Errorf("%w: set_active requires an atomic planning artifact", ErrWorkflowFocusPersistenceUnavailable)
	}
	for _, change := range resolved {
		if change.Add && change.ProjectID == "" {
			return nil, fmt.Errorf("%w: no owning project for task %q", ErrWorkflowFocusPersistenceUnavailable, change.TaskID)
		}
	}

	artifacts, err := builder(resolved)
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			return nil, fmt.Errorf("%w: planning artifact %q has no expected revision", ErrWorkflowFocusPersistenceUnavailable, artifact.Path)
		}
	}
	commits[0].artifacts = append(commits[0].artifacts, artifacts...)
	return resolved, nil
}
