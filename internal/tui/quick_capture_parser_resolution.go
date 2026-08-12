package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// resolveParent finds a task by exact short ID or a unique, case-insensitive
// title prefix. Prefix ambiguity is an error; task order must never decide
// which structural reference the user meant.
func resolveParent(ref string, project *models.Project) (*models.Task, error) {
	if project == nil {
		return nil, nil
	}
	if shortID, err := strconv.Atoi(ref); err == nil {
		for _, task := range project.Tasks {
			if task != nil && task.ShortID == shortID {
				return task, nil
			}
		}
	}

	refLower := strings.ToLower(ref)
	var matches []*models.Task
	for _, task := range project.Tasks {
		if task != nil && strings.HasPrefix(strings.ToLower(task.Title), refLower) {
			matches = append(matches, task)
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ShortID < matches[j].ShortID })
	labels := make([]string, len(matches))
	for i, task := range matches {
		labels[i] = fmt.Sprintf("#%d %s", task.ShortID, task.Title)
	}
	return nil, fmt.Errorf("parent reference %q is ambiguous: %s", ref, strings.Join(labels, ", "))
}

// resolveProject finds a non-archived project by exact slug. Autocomplete may
// fuzzy-filter display candidates, but committed structural references are
// stable canonical slugs rather than best-score guesses.
func resolveProject(ref string, allProjects []*models.Project) *models.Project {
	for _, project := range allProjects {
		if project != nil && !project.Archived && project.Slug == ref {
			return project
		}
	}
	return nil
}
