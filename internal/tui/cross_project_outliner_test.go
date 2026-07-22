package tui

import (
	"testing"
	"time"

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
		expandedNodes:     make(map[TaskIdentity]bool),
		collapsedSections: make(map[string]bool),
		filter:            NewFilterState(),
		sort:              NewSortState(),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		crossForceRebuild: true,
	}
	m.crossProjectRebuildRows()
	assertRowTitles(t, m.rows, []string{"Next task"})

	m.activePerspective = 4
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	assertRowTitles(t, m.rows, []string{"Inbox task"})
}

func TestCrossProjectDueDateLensOrdersWorkspaceBeforeProject(t *testing.T) {
	now := time.Now()
	laterDate := now.AddDate(0, 0, 10)
	overdueDate := now.AddDate(0, 0, -2)
	todayDate := now
	later := &models.Task{ID: "later", Title: "Later A", WorkflowState: models.StateReady, DueDate: &laterDate}
	undated := &models.Task{ID: "undated", Title: "Undated A", WorkflowState: models.StateReady}
	overdue := &models.Task{ID: "overdue", Title: "Overdue B", WorkflowState: models.StateReady, DueDate: &overdueDate}
	today := &models.Task{ID: "today", Title: "Today B", WorkflowState: models.StateReady, DueDate: &todayDate}
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Tasks: []*models.Task{later, undated}}
	beta := &models.Project{ID: "beta", Name: "Beta", Tasks: []*models.Task{overdue, today}}
	alpha.BuildTaskTree()
	beta.BuildTaskTree()
	m := NewOutlinerModel(nil, nil, newTestConfig())
	m.loadedProjects = []*models.Project{alpha, beta}
	m.crossForceRebuild = true
	m.ApplyOpenOptions(OutlinerOpenOptions{
		Focus: OutlinerAll, SetFocus: true,
		Lens: OutlinerLensDueDate, SetLens: true,
	})

	var got []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			got = append(got, "H:"+row.SectionName)
		} else if row.task != nil {
			got = append(got, row.task.ID+"@"+row.ProjectID)
		}
	}
	want := []string{"H:Overdue", "overdue@beta", "H:Today", "today@beta", "H:Later", "later@alpha", "H:No Date", "undated@alpha"}
	if len(got) != len(want) {
		t.Fatalf("due rows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("due rows = %v, want %v", got, want)
		}
	}
}

func TestCrossProjectSelectionUsesProjectQualifiedIdentity(t *testing.T) {
	alphaTask := &models.Task{ID: "shared", Title: "Alpha task"}
	betaTask := &models.Task{ID: "shared", Title: "Beta task"}
	alpha := &models.Project{ID: "alpha", Tasks: []*models.Task{alphaTask}}
	beta := &models.Project{ID: "beta", Tasks: []*models.Task{betaTask}}
	m := NewOutlinerModel(nil, nil, newTestConfig())
	m.loadedProjects = []*models.Project{alpha, beta}
	m.rows = []outlinerRow{{task: alphaTask, ProjectID: alpha.ID}, {task: betaTask, ProjectID: beta.ID}}
	m.multiSelectMode = true
	m.cursor = 0
	m.toggleCursorSelection()

	if !m.isRowSelected(m.rows[0]) || m.isRowSelected(m.rows[1]) {
		t.Fatalf("selection leaked across duplicate IDs: %+v", m.selectedTaskIdentities)
	}
	refs := m.getSelectedTaskProjectRefs()
	if len(refs) != 1 || refs[0].Project.ID != alpha.ID {
		t.Fatalf("selected refs = %+v, want alpha only", refs)
	}

	m.cursor = 1
	m.toggleCursorSelection()
	if refs = m.getSelectedTaskProjectRefs(); len(refs) != 2 {
		t.Fatalf("selected refs = %+v, want both qualified tasks", refs)
	}
}
