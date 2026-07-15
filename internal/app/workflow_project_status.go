package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/storage"
)

// WorkflowProjectStatusChange is the application-layer form of a project rule's
// catalog-owned status effect.
type WorkflowProjectStatusChange struct {
	ProjectID        string
	StatusIcon       string
	StatusIconSource string
}

// WorkflowProjectStatusArtifactBuilder applies project status changes to the
// workspace catalog and renders one revision-checked artifact for the grouped
// workflow transaction.
type WorkflowProjectStatusArtifactBuilder func([]WorkflowProjectStatusChange) ([]storage.ArtifactChange, error)

// WorkflowArtifactBuilders groups the durable configuration artifacts a
// standalone workflow evaluation may need.
type WorkflowArtifactBuilders struct {
	Focus         WorkflowFocusArtifactBuilder
	ProjectStatus WorkflowProjectStatusArtifactBuilder
}

func attachWorkflowProjectStatusArtifacts(
	commits []projectCommit,
	changes []WorkflowProjectStatusChange,
	builder WorkflowProjectStatusArtifactBuilder,
) ([]WorkflowProjectStatusChange, error) {
	resolved := normalizeWorkflowProjectStatusChanges(changes)
	if len(resolved) == 0 {
		return nil, nil
	}
	if len(commits) == 0 || builder == nil {
		return nil, fmt.Errorf("%w: project rules require an atomic catalog artifact", ErrWorkflowProjectStatusPersistenceUnavailable)
	}
	artifacts, err := builder(resolved)
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			return nil, fmt.Errorf("%w: catalog artifact %q has no expected revision", ErrWorkflowProjectStatusPersistenceUnavailable, artifact.Path)
		}
	}
	commits[0].artifacts = append(commits[0].artifacts, artifacts...)
	return resolved, nil
}

func normalizeWorkflowProjectStatusChanges(changes []WorkflowProjectStatusChange) []WorkflowProjectStatusChange {
	if len(changes) == 0 {
		return nil
	}
	resolved := make([]WorkflowProjectStatusChange, 0, len(changes))
	byProject := make(map[string]int, len(changes))
	for _, change := range changes {
		if change.ProjectID == "" || change.StatusIcon == "" {
			continue
		}
		if idx, ok := byProject[change.ProjectID]; ok {
			resolved[idx] = change
			continue
		}
		byProject[change.ProjectID] = len(resolved)
		resolved = append(resolved, change)
	}
	return resolved
}
