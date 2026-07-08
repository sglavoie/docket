package cmd

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// WithTaskUpdate encapsulates the common Load→FindTask→Update→Save pattern.
// It handles task lookup by identifier, captures state for workflow triggers,
// applies the update function, and saves with workflow rule evaluation.
// The update function receives the task and should return an error if the update fails.
// Returns the updated task on success for further inspection.
func WithTaskUpdate(taskIdentifier string, updateFn func(*models.Task) error) (*models.Task, error) {
	return WithTaskUpdateInProject(taskIdentifier, func(_ *models.Project, task *models.Task) error {
		return updateFn(task)
	})
}

// WithTaskUpdateInProject encapsulates the Load→FindTask→Update→Save pattern
// with access to the project. This is useful when the update function needs
// to interact with other project state (e.g., sections, other tasks).
// The update function receives the project and task and should return an error if the update fails.
// Returns the updated task on success for further inspection.
func WithTaskUpdateInProject(taskIdentifier string, updateFn func(*models.Project, *models.Task) error) (*models.Task, error) {
	return withTaskUpdateInProjectUndoAction(taskIdentifier, "Edit Task", updateFn)
}

func withTaskUpdateInProjectUndoAction(taskIdentifier, undoAction string, updateFn func(*models.Project, *models.Task) error) (*models.Task, error) {
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return nil, err
	}

	task, findErr := FindTaskByIdentifier(project, taskIdentifier)
	if findErr != nil {
		return nil, findErr
	}

	// Capture task states before changes for event-based workflow triggers
	prevStates := workflow.CaptureTaskStates(project.Tasks)

	// Apply the update
	if err := updateFn(project, task); err != nil {
		return nil, err
	}

	// Update timestamp
	task.UpdatedAt = time.Now()

	// Save with workflow rules evaluation and undo recording
	undoContext := fmt.Sprintf("#%d %s", task.ShortID, task.Title)
	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: configManager,
		PrevStates: prevStates, UndoAction: undoAction, UndoContext: undoContext,
	}); err != nil {
		return nil, err
	}

	return task, nil
}

// WithTaskRead encapsulates the common Load→FindTask→Read pattern for read-only commands.
// It loads the current project, finds the task, and passes it to the read function.
// No save is performed.
func WithTaskRead(taskIdentifier string, readFn func(*models.Project, *models.Task) error) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, findErr := FindTaskByIdentifier(project, taskIdentifier)
	if findErr != nil {
		return findErr
	}

	return readFn(project, task)
}
