package external

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/sglavoie/tt/internal/models"
)

// CopyTaskTitle copies the task title to the clipboard.
// Returns the copied text and any error.
func CopyTaskTitle(task *models.Task) (string, error) {
	text := task.Title
	return text, clipboard.WriteAll(text)
}

// CopyTaskTitles copies multiple task titles to the clipboard, one per line.
// Returns the copied text and any error.
func CopyTaskTitles(tasks []*models.Task) (string, error) {
	titles := make([]string, len(tasks))
	for i, task := range tasks {
		titles[i] = task.Title
	}
	text := strings.Join(titles, "\n")
	return text, clipboard.WriteAll(text)
}

// CopyTaskReference copies a task reference (e.g., "#123: Task title") to the clipboard.
// Returns the copied text and any error.
func CopyTaskReference(task *models.Task) (string, error) {
	text := fmt.Sprintf("#%d: %s", task.ShortID, task.Title)
	return text, clipboard.WriteAll(text)
}

// CopyTaskDetails copies task description to the clipboard.
// Returns the copied text and any error.
func CopyTaskDetails(task *models.Task) (string, error) {
	text := task.Description
	return text, clipboard.WriteAll(text)
}

// CopyTasksDetails copies descriptions for multiple tasks to the clipboard.
// Returns the copied text and any error.
func CopyTasksDetails(tasks []*models.Task) (string, error) {
	var parts []string
	for _, task := range tasks {
		parts = append(parts, task.Description)
	}
	text := strings.Join(parts, "\n---\n\n")
	return text, clipboard.WriteAll(text)
}

// CopyTaskTitleAndDescription copies task title and description to the clipboard.
// Format: title + blank line + description
// Returns the copied text and any error.
func CopyTaskTitleAndDescription(task *models.Task) (string, error) {
	text := task.Title + "\n\n" + task.Description
	return text, clipboard.WriteAll(text)
}

// CopyTasksTitleAndDescription copies multiple tasks' title and description to the clipboard.
// Each task is formatted as title + blank line + description, separated by "---".
// Returns the copied text and any error.
func CopyTasksTitleAndDescription(tasks []*models.Task) (string, error) {
	var parts []string
	for _, task := range tasks {
		parts = append(parts, task.Title+"\n\n"+task.Description)
	}
	text := strings.Join(parts, "\n---\n\n")
	return text, clipboard.WriteAll(text)
}
