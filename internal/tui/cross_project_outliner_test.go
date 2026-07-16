package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectPerspectivesUseSharedSectionMembership(t *testing.T) {
	next := &models.Task{ID: "next", Title: "Next task", WorkflowState: models.StateNext, Priority: 3}
	inbox := &models.Task{ID: "inbox", Title: "Inbox task", WorkflowState: models.StateInbox, Priority: 1, Tags: []string{"tagged"}}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{next, inbox}}
	project.BuildTaskTree()

	m := &OutlinerModel{
		activePerspective: 3,
		loadedProjects:    []*models.Project{project},
		expandedNodes:     make(map[string]bool),
		collapsedSections: make(map[string]bool),
		filter:            NewFilterState(),
		sort:              NewSortState(),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		crossForceRebuild: true,
	}
	m.crossProjectRebuildRows()
	assertRowTitles(t, m.rows, []string{"Next task"})

	m.activePerspective = 4
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	assertRowTitles(t, m.rows, []string{"Inbox task"})
}
