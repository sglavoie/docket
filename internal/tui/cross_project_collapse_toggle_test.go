package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossHandleEnterKeyCollapseTogglesRows(t *testing.T) {
	proj := &models.Project{
		ID:   "proj-toggle",
		Name: "Toggle",
		Tasks: []*models.Task{
			makeTask("t1", "Task one", models.SectionInbox),
			makeTask("t2", "Task two", models.SectionInbox),
		},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
	}
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	countTaskRows := func() int {
		n := 0
		for _, r := range m.rows {
			if r.isTaskRow() && r.task != nil {
				n++
			}
		}
		return n
	}

	initialTaskRows := countTaskRows()
	if initialTaskRows == 0 {
		t.Fatal("expected at least one task row after initial build")
	}

	headerIdx := -1
	for i, r := range m.rows {
		if r.IsSectionHeader {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 {
		t.Fatal("no section header row found; cannot test collapse toggle")
	}
	m.cursor = headerIdx

	_, _, handled := m.crossHandleEnterKey()
	if !handled {
		t.Fatal("crossHandleEnterKey returned handled=false on section header")
	}
	afterCollapse := countTaskRows()
	if afterCollapse >= initialTaskRows {
		t.Errorf("after collapse: want fewer than %d task rows, got %d", initialTaskRows, afterCollapse)
	}

	for i, r := range m.rows {
		if r.IsSectionHeader {
			m.cursor = i
			break
		}
	}

	_, _, handled = m.crossHandleEnterKey()
	if !handled {
		t.Fatal("crossHandleEnterKey returned handled=false on second toggle")
	}
	afterExpand := countTaskRows()
	if afterExpand != initialTaskRows {
		t.Errorf("after expand: want %d task rows, got %d", initialTaskRows, afterExpand)
	}
}
