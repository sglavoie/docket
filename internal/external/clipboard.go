package external

import (
	"github.com/atotto/clipboard"

	"github.com/sglavoie/tt/internal/models"
)

// CopyTaskTitle copies the task title to the clipboard.
// Returns the copied text and any error.
func CopyTaskTitle(task *models.Task) (string, error) {
	text := task.Title
	return text, clipboard.WriteAll(text)
}

// CopyTaskDetails copies task description to the clipboard.
// Returns the copied text and any error.
func CopyTaskDetails(task *models.Task) (string, error) {
	text := task.Description
	return text, clipboard.WriteAll(text)
}

// CopyTaskTitleAndDescription copies task title and description to the clipboard.
// Format: title + blank line + description
// Returns the copied text and any error.
func CopyTaskTitleAndDescription(task *models.Task) (string, error) {
	text := task.Title + "\n\n" + task.Description
	return text, clipboard.WriteAll(text)
}
