package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// FindTaskByIdentifier finds a task by short ID, UUID, or title prefix.
// Returns an error if no match is found or if a title prefix matches multiple tasks.
func FindTaskByIdentifier(project *models.Project, identifier string) (*models.Task, error) {
	// Guard empty identifier: HasPrefix("anything", "") below is always true,
	// so without this check an empty arg silently matches the first task.
	if identifier == "" {
		return nil, tterrors.NewTaskNotFoundError(identifier)
	}

	// Try as short ID first (must be purely numeric)
	if shortID, err := strconv.Atoi(identifier); err == nil {
		if task := project.GetTaskByShortID(shortID); task != nil {
			return task, nil
		}
	}

	// Try as UUID
	if task := project.GetTaskByID(identifier); task != nil {
		return task, nil
	}

	// Try as title prefix (case-insensitive)
	lowerID := strings.ToLower(identifier)
	var matches []*models.Task
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), lowerID) {
			matches = append(matches, task)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		// Build candidate list (show up to 5)
		limit := min(len(matches), 5)
		parts := make([]string, limit)
		for i := range limit {
			parts[i] = fmt.Sprintf("#%d %s", matches[i].ShortID, matches[i].Title)
		}
		candidates := strings.Join(parts, ", ")
		if len(matches) > 5 {
			candidates += fmt.Sprintf(", ... (%d more)", len(matches)-5)
		}
		return nil, tterrors.NewAmbiguousIdentifierError(identifier, candidates)
	}

	// No match found — suggest similar tasks using fuzzy matching
	if suggestions := suggestSimilarTasks(project.Tasks, identifier, 3); len(suggestions) > 0 {
		return nil, fmt.Errorf("%w: '%s'. Did you mean: %s", tterrors.ErrTaskNotFound, identifier, strings.Join(suggestions, ", "))
	}

	return nil, tterrors.NewTaskNotFoundError(identifier)
}

// ValidateParentTask finds a parent task by identifier and validates that the child type
// can be nested under it. Returns the parent task or an error.
func ValidateParentTask(project *models.Project, parentIdentifier string, childType models.TaskType) (*models.Task, error) {
	parent, findErr := FindTaskByIdentifier(project, parentIdentifier)
	if findErr != nil {
		return nil, findErr
	}

	if !childType.CanBeChildOf(parent.Type) {
		return nil, fmt.Errorf("%w: %s cannot be a child of %s", tterrors.ErrInvalidParent,
			childType.HeadingPrefix(), parent.Type.HeadingPrefix())
	}

	// Reject if adding a child would exceed the maximum nesting depth.
	// Without this check the task is created with the parent reference, but
	// BuildTaskTree silently detaches it at save time — misleading the user.
	if parent.Level+1 > models.MaxNestingDepth {
		return nil, fmt.Errorf("%w: nesting under #%d would exceed maximum depth of %d levels",
			tterrors.ErrInvalidParent, parent.ShortID, models.MaxNestingDepth)
	}

	return parent, nil
}

// suggestSimilarTasks returns formatted suggestions for tasks whose titles
// fuzzy-match the given identifier, sorted by score descending.
func suggestSimilarTasks(tasks []*models.Task, identifier string, limit int) []string {
	type scored struct {
		task  *models.Task
		score int
	}
	var candidates []scored
	for _, t := range tasks {
		if s := fuzzy.Score(t.Title, identifier); s > 0 {
			candidates = append(candidates, scored{task: t, score: s})
		}
	}
	// Sort by score descending
	slices.SortFunc(candidates, func(a, b scored) int {
		return b.score - a.score
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	suggestions := make([]string, len(candidates))
	for i, c := range candidates {
		suggestions[i] = fmt.Sprintf("#%d %s", c.task.ShortID, c.task.Title)
	}
	return suggestions
}
