package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

// GetRecentTasks returns the most recently updated tasks across all provided tasks.
func GetRecentTasks(tasks []*models.Task, limit int) []*models.Task {
	if len(tasks) == 0 {
		return nil
	}

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
	diff := time.Since(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	if diff < util.Day {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	// Use calendar-day comparison so that "47h ago" is "2 days ago" rather
	// than "yesterday". DaysBetween normalizes both ends to noon UTC, making
	// it DST-safe and immune to the wall-clock truncation issue.
	days := util.DaysBetween(t, time.Now())
	if days == 1 {
		return "yesterday"
	}
	return fmt.Sprintf("%d days ago", days)
}

// getTrashDir returns the path to the trash directory within the data directory.
func getTrashDir(dataDir string) string {
	return filepath.Join(dataDir, ".tt", "trash")
}

// TrashEntryExists checks if a project exists in the trash.
func TrashEntryExists(slug, dataDir string) bool {
	_, err := os.Stat(filepath.Join(getTrashDir(dataDir), slug))
	return err == nil
}

// RenameProjectDir renames a project's data directory when the slug changes.
// Updates project.Path to the new location. Returns nil if slugs are the same.
//
// Rejects empty slugs on either side: an empty slug would resolve to the
// projects/ directory itself, and renaming the entire projects/ tree onto
// (or from) a single project path would scramble every project's data.
func RenameProjectDir(project *models.Project, oldSlug, newSlug, dataDir string) error {
	if oldSlug == newSlug {
		return nil
	}
	if oldSlug == "" || newSlug == "" {
		return fmt.Errorf("project slug cannot be empty (old=%q, new=%q)", oldSlug, newSlug)
	}

	oldPath := filepath.Join(dataDir, "projects", oldSlug)
	newPath := filepath.Join(dataDir, "projects", newSlug)
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		project.Path = newPath
		return nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("directory '%s' already exists", newSlug)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to rename project directory: %w", err)
	}

	project.Path = newPath
	return nil
}

// MoveProjectToTrash moves a project directory to the trash folder.
// Callers should use PurgeFromTrash first if a same-slug entry might exist.
//
// Rejects an empty project slug: with slug="" the trash path collapses to
// the trash directory itself, and a poorly-formed call could rename an
// arbitrary project.Path on top of the trash root or vice versa.
func MoveProjectToTrash(project *models.Project, dataDir string) error {
	if project == nil || project.Slug == "" {
		return fmt.Errorf("project slug cannot be empty")
	}
	trashDir := getTrashDir(dataDir)
	trashPath := filepath.Join(trashDir, project.Slug)
	if err := os.MkdirAll(trashDir, DirPermissions); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}
	if err := os.Rename(project.Path, trashPath); err != nil {
		return fmt.Errorf("failed to move project to trash: %w", err)
	}
	return nil
}

// PurgeFromTrash permanently deletes a project from the trash.
// Returns nil if the project doesn't exist in trash.
//
// Rejects an empty slug: filepath.Join(trashDir, "") collapses to trashDir
// itself, so os.RemoveAll would wipe every trashed project rather than the
// requested one. Defense-in-depth alongside the cmd-layer guards added in
// 81a37d10.
func PurgeFromTrash(slug, dataDir string) error {
	if slug == "" {
		return fmt.Errorf("project slug cannot be empty")
	}
	if !TrashEntryExists(slug, dataDir) {
		return nil
	}
	return os.RemoveAll(filepath.Join(getTrashDir(dataDir), slug))
}

// RestoreProjectFromTrash moves a project directory from trash back to the data directory.
// Returns an error if the trash entry doesn't exist or if an active project with the same slug exists.
//
// Rejects an empty slug: with slug="" both paths collapse to their parent
// directories (trash root and projects root), so the rename would relocate
// the entire trash tree into projects/, scrambling every entry.
func RestoreProjectFromTrash(slug, dataDir string) error {
	if slug == "" {
		return fmt.Errorf("project slug cannot be empty")
	}
	trashPath := filepath.Join(getTrashDir(dataDir), slug)
	projectPath := filepath.Join(dataDir, "projects", slug)
	if !TrashEntryExists(slug, dataDir) {
		return fmt.Errorf("%w: '%s' not in trash", tterrors.ErrProjectNotFound, slug)
	}
	if _, err := os.Stat(projectPath); err == nil {
		return fmt.Errorf("%w: '%s' already exists; rename or delete it first", tterrors.ErrProjectExists, slug)
	}
	if err := os.MkdirAll(filepath.Dir(projectPath), DirPermissions); err != nil {
		return fmt.Errorf("failed to create projects directory: %w", err)
	}
	if err := os.Rename(trashPath, projectPath); err != nil {
		return fmt.Errorf("failed to restore project from trash: %w", err)
	}
	return nil
}

// ListTrashedProjects returns the slugs of all projects in the trash.
func ListTrashedProjects(dataDir string) ([]string, error) {
	entries, err := os.ReadDir(getTrashDir(dataDir))
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
