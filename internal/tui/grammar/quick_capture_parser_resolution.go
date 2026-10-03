package grammar

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// resolveParent finds the task a >parent reference names.
func resolveParent(ref string, project *models.Project) (*models.Task, error) {
	return ResolveStructuralReference(ref, project, "parent")
}

// resolveChild finds the task a <child reference names. The literal 0 resolves
// to nothing before any lookup runs: >0 detaches a task from its parent, so a
// <0 that adopted a task whose title happens to begin with a zero would read as
// that clear having quietly done something else.
func resolveChild(ref string, project *models.Project) (*models.Task, error) {
	if ref == "0" {
		return nil, nil
	}
	return ResolveStructuralReference(ref, project, "child")
}

// ResolveStructuralReference finds a task by exact short ID or a unique,
// case-insensitive title prefix. Prefix ambiguity is an error; task order must
// never decide which structural reference the user meant. The relationship names
// the sigil family in that error, so >parent and <child share one matching rule
// without reporting each other's vocabulary.
func ResolveStructuralReference(ref string, project *models.Project, relationship string) (*models.Task, error) {
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
	return nil, fmt.Errorf("%s reference %q is ambiguous: %s", relationship, ref, strings.Join(labels, ", "))
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

// suggestProjectSlugs offers up to three canonical slugs after a typo, without
// resolving the reference. Sorting keeps ambiguous suggestions independent of
// project load order. Archived projects cannot be capture destinations.
func suggestProjectSlugs(ref string, projects []*models.Project) []string {
	var suggestions []string
	seen := make(map[string]bool)
	for _, project := range projects {
		if project == nil || project.Archived || project.Slug == "" || seen[project.Slug] {
			continue
		}
		if projectSlugNearMatch(strings.ToLower(ref), strings.ToLower(project.Slug)) {
			seen[project.Slug] = true
			suggestions = append(suggestions, "%"+project.Slug)
		}
	}
	sort.Strings(suggestions)
	return suggestions[:min(3, len(suggestions))]
}

// projectSlugNearMatch accepts a case difference or one inserted, missing,
// replaced, or transposed character. A conservative bound avoids unrelated
// suggestions, and rune comparisons also handle non-ASCII slugs.
func projectSlugNearMatch(a, b string) bool {
	x, y := []rune(a), []rune(b)
	if len(x) == 0 || len(y) == 0 {
		return false
	}
	if len(x) > len(y) {
		x, y = y, x
	}
	if len(y)-len(x) > 1 {
		return false
	}
	for i := range x {
		if x[i] == y[i] {
			continue
		}
		if len(x) != len(y) {
			return string(x[i:]) == string(y[i+1:])
		}
		if string(x[i+1:]) == string(y[i+1:]) {
			return true
		}
		return i+1 < len(x) && x[i] == y[i+1] && x[i+1] == y[i] && string(x[i+2:]) == string(y[i+2:])
	}
	return true
}
