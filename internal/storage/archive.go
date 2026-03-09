package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/util"
)

const archiveFileName = "completed.md"
const archiveRotationThreshold int64 = 5 * 1024 * 1024 // 5MB

// GetArchiveFilePath returns the path to the project's archive file.
func GetArchiveFilePath(project *models.Project) string {
	return filepath.Join(project.Path, archiveFileName)
}

// LoadArchivedTasks reads completed.md (and any rotated files) for a project
// and returns all parsed tasks. Returns an empty slice if no archive exists.
func LoadArchivedTasks(project *models.Project) []*models.Task {
	var allTasks []*models.Task

	// Collect archive file paths: completed.md, completed.1.md, completed.2.md, ...
	var paths []string
	mainPath := filepath.Join(project.Path, archiveFileName)
	if _, err := os.Stat(mainPath); err == nil {
		paths = append(paths, mainPath)
	}
	for i := 1; ; i++ {
		rotatedPath := filepath.Join(project.Path, fmt.Sprintf("completed.%d.md", i))
		if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
			break
		}
		paths = append(paths, rotatedPath)
	}

	for _, p := range paths {
		tasks, err := parseArchiveFile(p)
		if err != nil {
			logging.Warn("failed to parse archive %s: %v", p, err)
			continue
		}
		allTasks = append(allTasks, tasks...)
	}

	return allTasks
}

// parseArchiveFile reads a single archive file and returns the tasks found in it.
func parseArchiveFile(path string) ([]*models.Task, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read archive file: %w", err)
	}

	result, err := ParseTasksMarkdownWithSections(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse archive: %w", err)
	}

	return result.Tasks, nil
}

// ArchiveTasks moves completed tasks from the project to the archive file.
// It removes the archived tasks from project.Tasks and appends them to completed.md.
// Returns the number of tasks archived and any error encountered.
func ArchiveTasks(project *models.Project) (int, error) {
	completedTasks := project.GetCompletedTasks()
	if len(completedTasks) == 0 {
		return 0, nil
	}

	archivePath := GetArchiveFilePath(project)

	// Rotate archive if needed before appending
	if err := rotateArchiveIfNeeded(project.Path, archiveRotationThreshold); err != nil {
		return 0, fmt.Errorf("failed to rotate archive: %w", err)
	}

	// Clean up attachment files for completed tasks
	for _, task := range completedTasks {
		_ = CleanupTaskAttachments(project, task)
	}

	// Generate markdown content for archived tasks
	content := WriteArchivedTasks(project.Name, completedTasks)

	// Append to archive file
	if err := appendToArchiveFile(archivePath, content); err != nil {
		return 0, fmt.Errorf("failed to append to archive: %w", err)
	}

	// Remove completed tasks from project
	removeCompletedTasks(project)

	// Save the modified project
	if err := SaveProject(project); err != nil {
		return 0, fmt.Errorf("failed to save project after archiving: %w", err)
	}

	return len(completedTasks), nil
}

// rotateArchiveIfNeeded checks if the archive file exceeds the threshold and rotates it.
// Rotation: completed.md -> completed.1.md, completed.1.md -> completed.2.md, etc.
func rotateArchiveIfNeeded(projectPath string, threshold int64) error {
	archivePath := filepath.Join(projectPath, archiveFileName)

	info, err := os.Stat(archivePath)
	if os.IsNotExist(err) {
		return nil // No archive file to rotate
	}
	if err != nil {
		return fmt.Errorf("failed to stat archive file: %w", err)
	}

	if info.Size() < threshold {
		return nil // File is under threshold
	}

	// Find the next available rotation number
	maxRotation := 0
	for i := 1; ; i++ {
		rotatedPath := filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i))
		if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
			break
		}
		maxRotation = i
	}

	// Shift existing rotated files up by one
	for i := maxRotation; i >= 1; i-- {
		oldPath := filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i))
		newPath := filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i+1))
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rotate %s to %s: %w", oldPath, newPath, err)
		}
	}

	// Move current archive to completed.1.md
	rotatedPath := filepath.Join(projectPath, "completed.1.md")
	if err := os.Rename(archivePath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rotate archive to %s: %w", rotatedPath, err)
	}

	return nil
}

// WriteArchivedTasks generates markdown content for archived tasks.
// Uses a simplified format without project sections metadata.
func WriteArchivedTasks(projectName string, tasks []*models.Task) string {
	if len(tasks) == 0 {
		return ""
	}

	var b strings.Builder

	// Write archive header with timestamp
	fmt.Fprintf(&b, "\n## Archived: %s\n", time.Now().Format(util.DateTimeFormat))
	fmt.Fprintf(&b, "<!-- project: %s, count: %d -->\n\n", projectName, len(tasks))

	// Write each task in a flat format (no hierarchy in archive)
	for _, task := range tasks {
		b.WriteString(writeArchivedTask(task))
		b.WriteString("\n")
	}

	return b.String()
}

// writeArchivedTask generates markdown for a single archived task.
func writeArchivedTask(task *models.Task) string {
	var b strings.Builder

	// Write heading (always level 3 for archived tasks)
	fmt.Fprintf(&b, "### %s: %s\n", task.Type.HeadingPrefix(), task.Title)

	// Write metadata
	b.WriteString(writeArchivedMetadata(task))
	b.WriteString("\n")

	// Write description if present
	if task.Description != "" {
		b.WriteString(task.Description)
		b.WriteString("\n")
	}

	// Preserve activity logs for future metrics (cycle time, blocked)
	if len(task.Activity) > 0 {
		b.WriteString("\n")
		b.WriteString(format.WriteActivity(task))
	}

	// Preserve comments to prevent data loss
	if len(task.Comments) > 0 {
		b.WriteString("\n")
		b.WriteString(format.WriteComments(task))
	}

	return b.String()
}

// writeArchivedMetadata generates the metadata comment for an archived task.
func writeArchivedMetadata(task *models.Task) string {
	var parts []string

	if task.ID != "" {
		parts = append(parts, fmt.Sprintf("id: %s", task.ID))
	}

	parts = append(parts, fmt.Sprintf("status: %s", task.Status))
	parts = append(parts, fmt.Sprintf("priority: %d", task.Priority))

	if len(task.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags: %s", strings.Join(task.Tags, ";")))
	}

	if task.Section != "" {
		parts = append(parts, fmt.Sprintf("section: %s", task.Section))
	}

	if !task.CreatedAt.IsZero() {
		parts = append(parts, fmt.Sprintf("created: %s", task.CreatedAt.Format(util.DateTimeFormat)))
	}

	if task.CompletedAt != nil {
		parts = append(parts, fmt.Sprintf("completed: %s", task.CompletedAt.Format(util.DateTimeFormat)))
	}

	if task.DueDate != nil {
		parts = append(parts, fmt.Sprintf("due: %s", task.DueDate.Format(util.DateFormat)))
	}

	if len(task.Attachments) > 0 {
		var attStrings []string
		for _, att := range task.Attachments {
			timestamp := fmt.Sprintf("@%d", att.AddedAt.Unix())
			if att.IsURL {
				attStrings = append(attStrings, "url:"+att.Filename+timestamp)
			} else {
				attStrings = append(attStrings, att.Filename+timestamp)
			}
		}
		parts = append(parts, fmt.Sprintf("attachments: %s", strings.Join(attStrings, ";")))
	}

	return fmt.Sprintf("<!-- %s -->", strings.Join(parts, ", "))
}

// appendToArchiveFile appends content to the archive file, creating it if necessary.
func appendToArchiveFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePermissions)
	if err != nil {
		return fmt.Errorf("failed to open archive file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to archive file: %w", err)
	}

	return nil
}

// removeCompletedTasks removes all completed tasks from the project's task list.
func removeCompletedTasks(project *models.Project) {
	var remaining []*models.Task
	for _, task := range project.Tasks {
		if !task.IsComplete() {
			remaining = append(remaining, task)
		}
	}
	project.Tasks = remaining
}
