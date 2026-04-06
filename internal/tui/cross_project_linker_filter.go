package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

func (m *CrossProjectLinkerModel) filterProjects() {
	query := strings.TrimSpace(strings.ToLower(m.SearchQuery()))
	if query == "" {
		m.filteredProjects = m.allProjects
		m.projectCursor = clamp(m.projectCursor, 0, len(m.filteredProjects)-1)
		return
	}

	type scoredProject struct {
		project *models.Project
		score   int
	}
	var scored []scoredProject

	for _, p := range m.allProjects {
		searchText := strings.ToLower(p.Name + " " + p.Context)
		score := fuzzy.ScoreTerms(searchText, query)
		if score > 0 {
			scored = append(scored, scoredProject{project: p, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.filteredProjects = make([]*models.Project, len(scored))
	for i, s := range scored {
		m.filteredProjects[i] = s.project
	}
	m.projectCursor = 0
}

func (m *CrossProjectLinkerModel) filterTasks() {
	query := strings.TrimSpace(strings.ToLower(m.SearchQuery()))
	if query == "" {
		m.filteredTasks = m.allTasks
		m.taskCursor = clamp(m.taskCursor, 0, len(m.filteredTasks)-1)
		return
	}

	type scoredTask struct {
		task  *models.Task
		score int
	}
	var scored []scoredTask

	for _, t := range m.allTasks {
		searchText := strings.ToLower(t.Title + " " + fmt.Sprintf("#%d", t.ShortID))
		score := fuzzy.ScoreTerms(searchText, query)
		if score > 0 {
			scored = append(scored, scoredTask{task: t, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.filteredTasks = make([]*models.Task, len(scored))
	for i, s := range scored {
		m.filteredTasks[i] = s.task
	}
	m.taskCursor = 0
}
