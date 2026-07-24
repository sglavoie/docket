package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestWeeklyReviewMultiSelectKeepsDuplicateTaskIDsIndependent(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta"}
	alphaTask := &models.Task{ID: "shared-id", Title: "Alpha task"}
	betaTask := &models.Task{ID: "shared-id", Title: "Beta task"}
	m := &WeeklyReviewModel{
		MultiSelectState:  MultiSelectState{multiSelectMode: true, selectedTaskIdentities: make(map[TaskIdentity]bool)},
		staleTasks:        []WeeklyReviewTask{{Task: alphaTask, Project: alpha}, {Task: betaTask, Project: beta}},
		collapsedSections: make(map[int]bool),
		tabs:              TabState{Names: weeklyReviewTabNames, Active: 1},
	}
	m.calculateNavigation()

	m.cursor = 1
	m.toggleTaskSelection()
	m.cursor = 2
	m.toggleTaskSelection()
	if got := m.getSelectedBulkItems(); len(got) != 2 {
		t.Fatalf("combined selection count = %d, want 2", len(got))
	}
	m.cursor = 1
	m.toggleTaskSelection()
	if got := m.getSelectedBulkItems(); len(got) != 1 || got[0].Project != beta {
		t.Fatalf("deselecting alpha affected beta: %+v", got)
	}
}
