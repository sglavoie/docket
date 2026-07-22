package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildProjectGroupedRowsIndependentSorting(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "p1",
			tasks: []*models.Task{
				makeTask("a1", "Alpha 1", ""),
				makeTask("a2", "Alpha 2", ""),
			},
		},
		{
			projectID: "p2",
			tasks:     []*models.Task{makeTask("b1", "Beta 1", "")},
		},
	}

	m.buildProjectGroupedRows(groups)

	var ids []string
	for _, row := range m.rows {
		if row.isTaskRow() {
			ids = append(ids, row.ProjectID)
		}
	}

	want := []string{"p1", "p1", "p2"}
	if len(ids) != len(want) {
		t.Fatalf("expected %d task rows, got %d", len(want), len(ids))
	}
	for i, wantID := range want {
		if ids[i] != wantID {
			t.Errorf("row[%d] ProjectID: want %q, got %q", i, wantID, ids[i])
		}
	}
}

func TestBuildProjectGroupedRowsEmptyGroups(t *testing.T) {
	m := minimalOutliner()

	m.buildProjectGroupedRows(nil)

	if len(m.rows) != 0 {
		t.Errorf("expected no rows for nil groups, got %d", len(m.rows))
	}
}

func TestCrossProjectRebuildRowsSortsNonAllGroupsByProjectName(t *testing.T) {
	alphaTask := makeTask("alpha-task", "Alpha active task", "")
	alphaTask.WorkflowState = models.StateInProgress
	alpha := &models.Project{
		ID:    "proj-alpha",
		Slug:  "alpha",
		Name:  "Alpha",
		Tasks: []*models.Task{alphaTask},
	}
	alpha.BuildTaskTree()

	betaTask := makeTask("beta-task", "Beta active task", "")
	betaTask.WorkflowState = models.StateInProgress
	beta := &models.Project{
		ID:    "proj-beta",
		Slug:  "beta",
		Name:  "Beta",
		Tasks: []*models.Task{betaTask},
	}
	beta.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		activePerspective: PerspectiveAll + 1,
		loadedProjects:    []*models.Project{beta, alpha},
	}

	m.crossProjectRebuildRows()

	var got []string
	for _, row := range m.rows {
		switch {
		case row.IsProjectHeader:
			got = append(got, "H:"+row.ProjectID)
		case row.IsSeparator:
			got = append(got, "|")
		default:
			got = append(got, row.ProjectID)
		}
	}

	want := []string{"H:proj-alpha", "proj-alpha", "H:proj-beta", "proj-beta"}
	if !slices.Equal(got, want) {
		t.Fatalf("rendered project order: want %v, got %v", want, got)
	}
}
