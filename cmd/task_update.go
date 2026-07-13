package cmd

import (
	"github.com/sglavoie/tt/internal/models"
)

// WithTaskRead encapsulates the common Load→FindTask→Read pattern for read-only commands.
// It loads the current project, finds the task, and passes it to the read function.
// No save is performed.
//
// The former WithTaskUpdate/WithTaskUpdateInProject write helpers are gone:
// task mutations persist through the application layer (newAppService) rather
// than the legacy cmd.SaveProject path.
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
