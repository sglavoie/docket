package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// GetRecentTasks returns the most recently updated tasks across all provided tasks.
func GetRecentTasks(tasks []*models.Task, limit int) []*models.Task {
	if len(tasks) == 0 || limit <= 0 {
		return nil
	}

	sorted := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			sorted = append(sorted, task)
		}
	}
	if len(sorted) == 0 {
		return nil
	}

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

	projectsDir := filepath.Join(dataDir, "projects")
	oldPath, err := SafeJoin(projectsDir, oldSlug)
	if err != nil {
		return fmt.Errorf("invalid project slug %q: %w", oldSlug, err)
	}
	newPath, err := SafeJoin(projectsDir, newSlug)
	if err != nil {
		return fmt.Errorf("invalid project slug %q: %w", newSlug, err)
	}
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
