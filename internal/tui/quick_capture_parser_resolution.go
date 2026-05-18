package tui

import (
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// resolveParent finds a task by short ID or title prefix.
func resolveParent(ref string, project *models.Project) *models.Task {
	if shortID, err := strconv.Atoi(ref); err == nil {
		for _, task := range project.Tasks {
			if task.ShortID == shortID {
				return task
			}
		}
	}

	refLower := strings.ToLower(ref)
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), refLower) {
			return task
		}
	}

	return nil
}

// resolveProject finds a project by slug (exact match) or fuzzy name match.
func resolveProject(ref string, allProjects []*models.Project) *models.Project {
	refLower := strings.ToLower(ref)
	for _, project := range allProjects {
		if !project.Archived && strings.ToLower(project.Slug) == refLower {
			return project
		}
	}

	type scoredProject struct {
		project *models.Project
		score   int
	}

	var scored []scoredProject
	for _, project := range allProjects {
		if project.Archived {
			continue
		}
		score := fuzzy.Score(project.Name+" "+project.Slug, ref)
		if score > 0 {
			scored = append(scored, scoredProject{project: project, score: score})
		}
	}

	if len(scored) == 0 {
		return nil
	}

	best := scored[0]
	for _, candidate := range scored[1:] {
		if candidate.score > best.score {
			best = candidate
		}
	}
	return best.project
}

// parsePriorityToken converts a priority token (numeric, word alias, or short prefix) to 1–5.
// Word mappings: critical/urgent/high→1, normal/medium→3, low/lowest→5.
// Short prefixes: c/cr/u/ur/h/hi→1, n/no/m/me/med→3, l/lo→5.
func parsePriorityToken(token string) int {
	abbrev := map[string]int{
		"c": 1, "cr": 1,
		"u": 1, "ur": 1,
		"h": 1, "hi": 1,
		"n": 3, "no": 3,
		"m": 3, "me": 3, "med": 3,
		"l": 5, "lo": 5,
	}

	lower := strings.ToLower(token)
	if priority, ok := abbrev[lower]; ok {
		return priority
	}

	switch lower {
	case "critical", "urgent", "high":
		return 1
	case "normal", "medium":
		return 3
	case "low", "lowest":
		return 5
	default:
		priority, _ := strconv.Atoi(token)
		return priority
	}
}
