package storage

import (
	"fmt"
	"path/filepath"

	"github.com/sglavoie/tt/internal/models"
)

// getUndoDir returns the path to the .undo directory for a project.
func getUndoDir(project *models.Project) string {
	return filepath.Join(project.Path, undoDirName)
}

// getUndoMetaPath returns the path to the meta.json file.
func getUndoMetaPath(project *models.Project) string {
	return filepath.Join(getUndoDir(project), undoMetaFileName)
}

// getSnapshotPath returns the path to a specific snapshot file.
func getSnapshotPath(project *models.Project, num int) string {
	return filepath.Join(getUndoDir(project), fmt.Sprintf("%04d.md", num))
}

// getRedoSnapshotPath returns the path to a specific redo snapshot file.
func getRedoSnapshotPath(project *models.Project, num int) string {
	return filepath.Join(getUndoDir(project), fmt.Sprintf("redo_%04d.md", num))
}
