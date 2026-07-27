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
