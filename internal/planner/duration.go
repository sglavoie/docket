package planner

import (
	"github.com/sglavoie/tt/internal/models"
)

// Effort-to-day multipliers. These are intentionally simple constants; a future
// configuration layer can override them without changing the algorithm.
const (
	effortSmallDays   = 0.5
	effortMediumDays  = 1.0
	effortLargeDays   = 3.0
	effortUnknownDays = 1.0
	storyPointToDays  = 0.5 // 1 story point ≈ half a working day
	defaultDays       = 1.0
)

// effortDays maps an EffortEstimate to its day estimate.
func effortDays(e models.EffortEstimate) float64 {
	switch e {
	case models.EffortSmall:
		return effortSmallDays
	case models.EffortMedium:
		return effortMediumDays
	case models.EffortLarge:
		return effortLargeDays
	default:
		return effortUnknownDays
	}
}

// EstimateDuration returns the estimated duration for a task in working days.
//
// Priority order:
//  1. task.EstimatedDays when > 0 (explicit override)
//  2. TaskAttrs.Effort or IdeaAttrs.Effort via the multiplier table
//  3. StoryAttrs.StoryPoints * storyPointToDays when StoryPoints > 0
//  4. defaultDays (1.0) as the final fallback
func EstimateDuration(task *models.Task) float64 {
	if task == nil {
		return defaultDays
	}
	if task.EstimatedDays > 0 {
		return task.EstimatedDays
	}
	if task.TaskAttrs != nil && task.TaskAttrs.Effort != "" {
		return effortDays(task.TaskAttrs.Effort)
	}
	if task.IdeaAttrs != nil && task.IdeaAttrs.Effort != "" {
		return effortDays(task.IdeaAttrs.Effort)
	}
	if task.StoryAttrs != nil && task.StoryAttrs.StoryPoints > 0 {
		return float64(task.StoryAttrs.StoryPoints) * storyPointToDays
	}
	return defaultDays
}

// TotalRemainingDays returns the sum of EstimateDuration for all incomplete
// direct children of parent. It does not recurse into grandchildren.
// Returns 0 when parent is nil or has no children.
func TotalRemainingDays(parent *models.Task) float64 {
	if parent == nil {
		return 0
	}
	var total float64
	for _, child := range parent.Children {
		if child == nil {
			continue
		}
		if !child.IsComplete() {
			total += EstimateDuration(child)
		}
	}
	return total
}
