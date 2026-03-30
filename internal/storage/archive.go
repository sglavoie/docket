package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	matches, err := filepath.Glob(filepath.Join(project.Path, "completed.*.md"))
	if err == nil {
		type numberedPath struct {
			num  int
			path string
		}
		var numbered []numberedPath
		for _, m := range matches {
			base := filepath.Base(m)
			// Extract the number between "completed." and ".md"
			numStr := strings.TrimPrefix(base, "completed.")
			numStr = strings.TrimSuffix(numStr, ".md")
			n, err := strconv.Atoi(numStr)
			if err != nil {
				continue // skip non-numeric matches
			}
			numbered = append(numbered, numberedPath{num: n, path: m})
		}
		sort.Slice(numbered, func(i, j int) bool {
			return numbered[i].num < numbered[j].num
		})
		for _, np := range numbered {
			paths = append(paths, np.path)
		}
	}

	for _, p := range paths {
		tasks, err := parseArchiveFile(p)
		if err != nil {
			logging.Warn("skipping corrupt archive %s: %v", p, err)
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

	// Record archive size before appending so we can rollback on failure
	var archiveSizeBefore int64
	if info, err := os.Stat(archivePath); err == nil {
		archiveSizeBefore = info.Size()
	}

	// Generate markdown content for archived tasks
	content := WriteArchivedTasks(project.Name, completedTasks)

	// Append to archive file
	if err := appendToArchiveFile(archivePath, content); err != nil {
		return 0, fmt.Errorf("failed to append to archive: %w", err)
	}

	// Remove completed tasks from project and fix orphaned children
	archivedIDs := make(map[string]bool, len(completedTasks))
	for _, t := range completedTasks {
		archivedIDs[t.ID] = true
	}
	removeCompletedTasks(project)
	clearOrphanedParentRefs(project, archivedIDs)

	// Save the modified project; rollback archive append on failure
	if err := SaveProject(project); err != nil {
		// Truncate archive back to pre-append size
		if truncErr := os.Truncate(archivePath, archiveSizeBefore); truncErr != nil {
			logging.Warn("failed to rollback archive after save failure: %v", truncErr)
		}
		// Reload project from disk to avoid in-memory/on-disk divergence
		if loadErr := LoadProject(project); loadErr != nil {
			logging.Warn("failed to reload project after save failure, restoring in-memory tasks: %v", loadErr)
			project.Tasks = append(project.Tasks, completedTasks...)
		}
		return 0, fmt.Errorf("failed to save project after archiving: %w", err)
	}

	// Clean up attachment files only after both archive and save succeeded
	for _, task := range completedTasks {
		if err := CleanupTaskAttachments(project, task); err != nil {
			logging.Warn("failed to clean up attachments for task %s: %v", task.ID, err)
		}
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

	// Track completed renames for rollback on failure
	type rename struct{ from, to string }
	var completed []rename

	rollback := func() {
		// Reverse renames in LIFO order (best-effort)
		for i := len(completed) - 1; i >= 0; i-- {
			r := completed[i]
			if rbErr := os.Rename(r.to, r.from); rbErr != nil {
				logging.Warn("archive rotation rollback failed: %s -> %s: %v", r.to, r.from, rbErr)
			}
		}
	}

	// Shift existing rotated files up by one
	for i := maxRotation; i >= 1; i-- {
		oldPath := filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i))
		newPath := filepath.Join(projectPath, fmt.Sprintf("completed.%d.md", i+1))
		if err := os.Rename(oldPath, newPath); err != nil {
			rollback()
			return fmt.Errorf("failed to rotate %s to %s: %w", oldPath, newPath, err)
		}
		completed = append(completed, rename{from: oldPath, to: newPath})
	}

	// Move current archive to completed.1.md
	rotatedPath := filepath.Join(projectPath, "completed.1.md")
	if err := os.Rename(archivePath, rotatedPath); err != nil {
		rollback()
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

	// Write metadata — use the canonical WriteMetadata to preserve ALL fields
	// (custom_section, depends, related, checklist, stage, source, url,
	// score dimensions, type-specific attrs, code ref attachments, etc.)
	b.WriteString(format.WriteMetadata(task))
	b.WriteString("\n")

	// Write description (escape heading-like lines to prevent parser corruption)
	if task.Description != "" {
		b.WriteString(escapeDescription(task.Description))
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

	// Sync to disk before returning so that a subsequent crash doesn't
	// lose the archived data (the caller truncates on save failure).
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync archive file: %w", err)
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

// clearOrphanedParentRefs clears ParentID on any remaining task whose parent
// was archived (removed). Without this, children of archived parents would have
// dangling ParentID references and be silently promoted to root by BuildTaskTree.
// Also removes DependsOn and RelatedTo references to archived tasks.
func clearOrphanedParentRefs(project *models.Project, removedIDs map[string]bool) {
	for _, task := range project.Tasks {
		if task.ParentID != nil && removedIDs[*task.ParentID] {
			task.ParentID = nil
		}
		if len(task.DependsOn) > 0 {
			clean := task.DependsOn[:0]
			for _, depID := range task.DependsOn {
				if !removedIDs[depID] {
					clean = append(clean, depID)
				}
			}
			task.DependsOn = clean
		}
		if len(task.RelatedTo) > 0 {
			clean := task.RelatedTo[:0]
			for _, relID := range task.RelatedTo {
				if !removedIDs[relID] {
					clean = append(clean, relID)
				}
			}
			task.RelatedTo = clean
		}
	}
}
