package storage

import (
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/models"
)

// GetSnapshotContent reads the content of a specific snapshot.
func GetSnapshotContent(project *models.Project, num int) (string, error) {
	snapshotPath := getSnapshotPath(project, num)

	content, err := os.ReadFile(snapshotPath)
	if err != nil {
		return "", fmt.Errorf("failed to read snapshot %d: %w", num, err)
	}

	return string(content), nil
}

// GetRedoSnapshotContent reads the content of a specific redo snapshot.
func GetRedoSnapshotContent(project *models.Project, num int) (string, error) {
	redoPath := getRedoSnapshotPath(project, num)

	content, err := os.ReadFile(redoPath)
	if err != nil {
		return "", fmt.Errorf("failed to read redo snapshot %d: %w", num, err)
	}

	return string(content), nil
}

// GetCurrentContent reads the current tasks.md content.
func GetCurrentContent(project *models.Project) (string, error) {
	tasksPath := GetTasksFilePath(project)

	content, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read current tasks: %w", err)
	}

	return string(content), nil
}
