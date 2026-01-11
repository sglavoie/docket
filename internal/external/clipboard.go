package external

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/sglavoie/tt/internal/models"
)

// CopyToClipboard copies text to the system clipboard.
func CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

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
