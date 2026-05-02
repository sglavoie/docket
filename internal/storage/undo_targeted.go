package storage

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// buildTargetedRestore parses the current content, replaces only the targeted
// tasks with their versions from the snapshot, and re-serializes.
func buildTargetedRestore(currentContent string, snapshotResult *ParseResult, taskIDs []string) (string, error) {
	currentResult, err := ParseTasksMarkdownWithSections(currentContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse current content: %w", err)
	}

	// Build snapshot task lookup
	snapshotMap := make(map[string]*models.Task, len(snapshotResult.Tasks))
	for _, t := range snapshotResult.Tasks {
		snapshotMap[t.ID] = t
	}

	// Build set of targeted task IDs for O(1) lookup
	targetSet := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		targetSet[id] = true
	}

	// Remove tasks that were newly created (exist in current but not in
	// snapshot). Undoing a creation means the task should disappear.
	filtered := make([]*models.Task, 0, len(currentResult.Tasks))
	for _, t := range currentResult.Tasks {
		if targetSet[t.ID] && snapshotMap[t.ID] == nil {
			continue // Task was created after snapshot; remove it
		}
		filtered = append(filtered, t)
	}
	currentResult.Tasks = filtered

	// Build a set of current task IDs for quick lookup
	currentIDs := make(map[string]bool, len(currentResult.Tasks))
	for _, t := range currentResult.Tasks {
		currentIDs[t.ID] = true
	}

	// Replace targeted tasks' fields, or re-add tasks that were deleted
	for _, taskID := range taskIDs {
		snapTask, ok := snapshotMap[taskID]
		if !ok {
			continue // Task didn't exist in snapshot and was already removed above
		}
		if !currentIDs[taskID] {
			// Task was deleted after snapshot; re-add it from the snapshot
			currentResult.Tasks = append(currentResult.Tasks, snapTask)
			continue
		}
		for _, curTask := range currentResult.Tasks {
			if curTask.ID == taskID {
				restoreTaskFields(curTask, snapTask)
				break
			}
		}
	}

	return WriteTasksMarkdownWithSections(
		currentResult.ProjectName,
		currentResult.Tasks,
		currentResult.Sections,
		currentResult.SectionColors,
	), nil
}

// restoreTaskFields copies all persistent fields from src to dst, preserving
// only the ID (must be identical) and computed tree pointers (Children, Parent,
// Project, Level) which are rebuilt by BuildTaskTree.
func restoreTaskFields(dst, src *models.Task) {
	dst.ShortID = src.ShortID
	dst.Type = src.Type
	dst.Title = src.Title
	dst.Description = src.Description
	dst.Status = src.Status
	dst.Priority = src.Priority
	dst.ParentID = src.ParentID
	dst.Section = src.Section
	dst.CustomSection = src.CustomSection
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
	dst.CompletedAt = src.CompletedAt
	dst.DueDate = src.DueDate
	dst.HasTime = src.HasTime
	dst.Active = src.Active
	dst.Pinned = src.Pinned
	dst.PinnedAt = src.PinnedAt
	dst.Dimmed = src.Dimmed
	dst.SnoozedUntil = src.SnoozedUntil
	dst.Stage = src.Stage
	dst.StartDate = src.StartDate
	dst.EstimatedDays = src.EstimatedDays
	dst.Source = src.Source
	dst.URL = src.URL

	// Deep copy slices and maps
	dst.Tags = copyStringSlice(src.Tags)
	dst.DependsOn = copyStringSlice(src.DependsOn)
	dst.RelatedTo = copyStringSlice(src.RelatedTo)

	// Copy complex types by reference (parsed fresh from snapshot)
	dst.Comments = src.Comments
	dst.Recurrence = src.Recurrence
	dst.Attachments = src.Attachments
	dst.Activity = src.Activity
	dst.Checklist = src.Checklist
	dst.ScoreDimensions = src.ScoreDimensions
	dst.BugAttrs = src.BugAttrs
	dst.IdeaAttrs = src.IdeaAttrs
	dst.TaskAttrs = src.TaskAttrs
	dst.StoryAttrs = src.StoryAttrs
	dst.EpicAttrs = src.EpicAttrs
}

func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	return c
}
