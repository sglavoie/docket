package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// getChildrenInOrder returns tasks that are children of the given parent ID,
// preserving their original order in the tasks slice.
func getChildrenInOrder(parentID string, tasks []*models.Task) []*models.Task {
	var children []*models.Task
	for _, task := range tasks {
		if task.ParentID != nil && *task.ParentID == parentID {
			children = append(children, task)
		}
	}
	return children
}

const tasksFileName = "tasks.md"
const backupFileName = ".tasks.md.bak"

// LoadProject loads tasks from a project's tasks.md file.
func LoadProject(project *models.Project) error {
	tasksPath := filepath.Join(project.Path, tasksFileName)

	content, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		// No tasks file yet, initialize empty
		project.Tasks = []*models.Task{}
		project.NextShortID = 1
		project.Sections = []string{models.SectionNext}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read tasks file: %w", err)
	}

	result, err := ParseTasksMarkdownWithSections(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse tasks: %w", err)
	}

	project.Tasks = result.Tasks
	project.Sections = result.Sections
	project.SectionColors = result.SectionColors

	// Determine next short ID
	maxShortID := 0
	for _, task := range result.Tasks {
		if task.ShortID > maxShortID {
			maxShortID = task.ShortID
		}
	}
	project.NextShortID = maxShortID + 1

	// Build the tree structure
	project.BuildTaskTree()

	return nil
}

// SaveProject saves a project's tasks to its tasks.md file.
func SaveProject(project *models.Project) error {
	if err := os.MkdirAll(project.Path, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	content := WriteTasksMarkdownWithSections(project.Name, project.Tasks, project.GetSections(), project.SectionColors)

	tasksPath := filepath.Join(project.Path, tasksFileName)
	if err := os.WriteFile(tasksPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	return nil
}

// WriteTasksMarkdown generates markdown content from tasks.
// Tasks are written in their slice order, preserving user-defined ordering.
func WriteTasksMarkdown(projectName string, tasks []*models.Task) string {
	return WriteTasksMarkdownWithSections(projectName, tasks, nil, nil)
}

// WriteTasksMarkdownWithSections generates markdown content from tasks and sections.
// Tasks are written in their slice order, preserving user-defined ordering.
func WriteTasksMarkdownWithSections(projectName string, tasks []*models.Task, sections []string, sectionColors map[string]string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Project: %s\n", projectName))

	// Write sections metadata with optional colors
	// Format: "<!-- sections: Backlog;Active:green;Done:blue -->"
	if len(sections) > 0 {
		var sectionParts []string
		for _, name := range sections {
			if sectionColors != nil {
				if color, ok := sectionColors[name]; ok && color != "" && color != "gray" {
					sectionParts = append(sectionParts, name+":"+color)
				} else {
					sectionParts = append(sectionParts, name)
				}
			} else {
				sectionParts = append(sectionParts, name)
			}
		}
		b.WriteString(fmt.Sprintf("<!-- sections: %s -->\n", strings.Join(sectionParts, ";")))
	}
	b.WriteString("\n")

	// Collect root tasks in slice order (preserving user order)
	var rootTasks []*models.Task
	for _, task := range tasks {
		if task.ParentID == nil {
			rootTasks = append(rootTasks, task)
		}
	}

	// Write each root task and its children (children fetched in slice order)
	for i, task := range rootTasks {
		writeTaskPreservingOrder(&b, task, tasks, 2)
		if i < len(rootTasks)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// writeTaskPreservingOrder writes a single task and its children to the builder,
// preserving the original slice order for children.
func writeTaskPreservingOrder(b *strings.Builder, task *models.Task, allTasks []*models.Task, level int) {
	// Write heading
	hashes := strings.Repeat("#", level)
	b.WriteString(fmt.Sprintf("%s %s: %s\n", hashes, task.Type.HeadingPrefix(), task.Title))

	// Write metadata
	b.WriteString(writeMetadata(task))
	b.WriteString("\n")

	// Write description
	if task.Description != "" {
		b.WriteString(task.Description)
		b.WriteString("\n")
	}

	// Write comments block if task has comments
	if len(task.Comments) > 0 {
		b.WriteString("\n")
		b.WriteString(writeComments(task))
	}

	// Get children in their original slice order (not sorted by type)
	children := getChildrenInOrder(task.ID, allTasks)

	// Write children
	for _, child := range children {
		b.WriteString("\n")
		writeTaskPreservingOrder(b, child, allTasks, level+1)
	}
}

// writeMetadata generates the metadata comment for a task.
func writeMetadata(task *models.Task) string {
	var parts []string

	if task.ID != "" {
		parts = append(parts, fmt.Sprintf("id: %s", task.ID))
	}

	// Notes don't have status, priority, or tags
	if !task.IsNote() {
		parts = append(parts, fmt.Sprintf("status: %s", task.Status))
		parts = append(parts, fmt.Sprintf("priority: %d", task.Priority))

		if len(task.Tags) > 0 {
			parts = append(parts, fmt.Sprintf("tags: %s", strings.Join(task.Tags, ";")))
		}
	}

	if task.Section != "" {
		parts = append(parts, fmt.Sprintf("section: %s", task.Section))
	}

	if !task.CreatedAt.IsZero() {
		parts = append(parts, fmt.Sprintf("created: %s", task.CreatedAt.Format(util.DateTimeFormat)))
	}

	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
		parts = append(parts, fmt.Sprintf("updated: %s", task.UpdatedAt.Format(util.DateTimeFormat)))
	}

	if task.CompletedAt != nil {
		parts = append(parts, fmt.Sprintf("completed: %s", task.CompletedAt.Format(util.DateTimeFormat)))
	}

	if task.DueDate != nil {
		parts = append(parts, fmt.Sprintf("due: %s", task.DueDate.Format(util.DateFormat)))
	}

	if len(task.DependsOn) > 0 {
		parts = append(parts, fmt.Sprintf("depends: %s", strings.Join(task.DependsOn, ";")))
	}

	if task.Pinned {
		parts = append(parts, "pinned: true")
	}

	if task.Active {
		parts = append(parts, "active: true")
	}

	if task.Recurrence != nil {
		parts = append(parts, fmt.Sprintf("recur: %s", formatRecurrence(task.Recurrence)))
	}

	return fmt.Sprintf("<!-- %s -->", strings.Join(parts, ", "))
}

// formatRecurrence converts a RecurrencePattern to its string representation for storage.
func formatRecurrence(r *models.RecurrencePattern) string {
	if r == nil {
		return ""
	}

	// Use compact format: Nd, Nw, Nm, Ny
	var unit string
	switch r.Unit {
	case models.RecurrenceDaily:
		unit = "d"
	case models.RecurrenceWeekly:
		unit = "w"
	case models.RecurrenceMonthly:
		unit = "m"
	case models.RecurrenceYearly:
		unit = "y"
	default:
		unit = "d"
	}

	base := fmt.Sprintf("%d%s", r.Interval, unit)

	// Add weekdays for weekly patterns
	if r.Unit == models.RecurrenceWeekly && len(r.Weekdays) > 0 {
		dayNames := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
		var days []string
		for _, d := range r.Weekdays {
			if d >= 0 && d < 7 {
				days = append(days, dayNames[d])
			}
		}
		if len(days) > 0 {
			return base + ":" + strings.Join(days, ",")
		}
	}

	return base
}

// writeComments generates the comments block for a task.
func writeComments(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<!-- COMMENTS -->\n")

	for _, comment := range task.Comments {
		b.WriteString(fmt.Sprintf("<!-- comment: %s -->\n", comment.CreatedAt.Format(util.DateTimeFormat)))
		// Escape HTML comment markers to prevent parser corruption
		escapedText := escapeCommentText(comment.Text)
		b.WriteString(escapedText)
		b.WriteString("\n")
	}

	b.WriteString("<!-- /COMMENTS -->\n")
	return b.String()
}

// escapeCommentText escapes HTML comment markers in comment text
// to prevent parser corruption when the text contains <!-- or -->.
func escapeCommentText(text string) string {
	text = strings.ReplaceAll(text, "<!--", "&lt;!--")
	text = strings.ReplaceAll(text, "-->", "--&gt;")
	return text
}

// unescapeCommentText reverses the escaping done by escapeCommentText.
func unescapeCommentText(text string) string {
	text = strings.ReplaceAll(text, "&lt;!--", "<!--")
	text = strings.ReplaceAll(text, "--&gt;", "-->")
	return text
}

// GetTasksFilePath returns the path to a project's tasks.md file.
func GetTasksFilePath(project *models.Project) string {
	return filepath.Join(project.Path, tasksFileName)
}

// GetBackupFilePath returns the path to a project's backup file.
func GetBackupFilePath(project *models.Project) string {
	return filepath.Join(project.Path, backupFileName)
}

// BackupExists checks if a backup file exists for the project.
func BackupExists(project *models.Project) bool {
	backupPath := GetBackupFilePath(project)
	_, err := os.Stat(backupPath)
	return !os.IsNotExist(err)
}

// BackupProject creates a backup of the current tasks.md file.
// Only creates a backup if tasks.md exists (no backup for new projects).
func BackupProject(project *models.Project) error {
	tasksPath := GetTasksFilePath(project)
	backupPath := GetBackupFilePath(project)

	// Only backup if the tasks file exists
	content, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		return nil // No tasks file to backup
	}
	if err != nil {
		return fmt.Errorf("failed to read tasks file for backup: %w", err)
	}

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// RestoreFromBackup swaps the current tasks file with the backup.
// This makes the operation reversible - running restore again undoes the restore.
func RestoreFromBackup(project *models.Project) error {
	tasksPath := GetTasksFilePath(project)
	backupPath := GetBackupFilePath(project)

	// Read backup content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Validate backup content is valid markdown before restoring
	_, err = ParseTasksMarkdownWithSections(string(backupContent))
	if err != nil {
		return fmt.Errorf("backup file is invalid: %w", err)
	}

	// Read current content (may not exist if tasks were deleted)
	currentContent, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		currentContent = []byte{}
	} else if err != nil {
		return fmt.Errorf("failed to read current tasks file: %w", err)
	}

	// Write backup content to main file
	if err := os.WriteFile(tasksPath, backupContent, 0644); err != nil {
		return fmt.Errorf("failed to restore tasks file: %w", err)
	}

	// Write current content to backup (for reversibility)
	if err := os.WriteFile(backupPath, currentContent, 0644); err != nil {
		return fmt.Errorf("failed to update backup file: %w", err)
	}

	return nil
}

// ProjectExists checks if a project directory and tasks file exist.
func ProjectExists(projectPath string) bool {
	tasksPath := filepath.Join(projectPath, tasksFileName)
	_, err := os.Stat(tasksPath)
	return !os.IsNotExist(err)
}

// CreateProjectDir creates the project directory structure.
func CreateProjectDir(project *models.Project) error {
	if err := os.MkdirAll(project.Path, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Create initial tasks.md
	return SaveProject(project)
}

// DeleteProjectDir removes a project's directory.
func DeleteProjectDir(projectPath string) error {
	return os.RemoveAll(projectPath)
}

// GetRecentTasks returns the most recently updated tasks across all provided tasks.
func GetRecentTasks(tasks []*models.Task, limit int) []*models.Task {
	if len(tasks) == 0 {
		return nil
	}

	// Create a copy to sort
	sorted := make([]*models.Task, len(tasks))
	copy(sorted, tasks)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})

	if limit > len(sorted) {
		limit = len(sorted)
	}

	return sorted[:limit]
}

// FormatRelativeTime returns a human-readable relative time string.
func FormatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

const trashDirName = ".trash"

// GetTrashDir returns the path to the trash directory within the data directory.
func GetTrashDir(dataDir string) string {
	return filepath.Join(dataDir, trashDirName)
}

// TrashEntryExists checks if a project exists in the trash.
func TrashEntryExists(slug, dataDir string) bool {
	trashPath := filepath.Join(GetTrashDir(dataDir), slug)
	_, err := os.Stat(trashPath)
	return !os.IsNotExist(err)
}

// MoveProjectToTrash moves a project directory to the trash folder.
// Callers should use PurgeFromTrash first if a same-slug entry might exist.
func MoveProjectToTrash(project *models.Project, dataDir string) error {
	trashDir := GetTrashDir(dataDir)
	trashPath := filepath.Join(trashDir, project.Slug)

	// Create trash directory if needed
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Move project directory to trash
	if err := os.Rename(project.Path, trashPath); err != nil {
		return fmt.Errorf("failed to move project to trash: %w", err)
	}

	return nil
}

// PurgeFromTrash permanently deletes a project from the trash.
// Returns nil if the project doesn't exist in trash.
func PurgeFromTrash(slug, dataDir string) error {
	if !TrashEntryExists(slug, dataDir) {
		return nil
	}
	trashPath := filepath.Join(GetTrashDir(dataDir), slug)
	return os.RemoveAll(trashPath)
}

// RestoreProjectFromTrash moves a project directory from trash back to the data directory.
// Returns an error if the trash entry doesn't exist or if an active project with the same slug exists.
func RestoreProjectFromTrash(slug, dataDir string) error {
	trashPath := filepath.Join(GetTrashDir(dataDir), slug)
	projectPath := filepath.Join(dataDir, slug)

	// Check if trash entry exists
	if !TrashEntryExists(slug, dataDir) {
		return fmt.Errorf("project '%s' not found in trash", slug)
	}

	// Check if active project exists
	if _, err := os.Stat(projectPath); !os.IsNotExist(err) {
		return fmt.Errorf("active project '%s' already exists; rename or delete it first", slug)
	}

	// Move from trash back to data directory
	if err := os.Rename(trashPath, projectPath); err != nil {
		return fmt.Errorf("failed to restore project from trash: %w", err)
	}

	return nil
}

// ListTrashedProjects returns the slugs of all projects in the trash.
func ListTrashedProjects(dataDir string) ([]string, error) {
	trashDir := GetTrashDir(dataDir)

	entries, err := os.ReadDir(trashDir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read trash directory: %w", err)
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			projects = append(projects, entry.Name())
		}
	}

	return projects, nil
}
