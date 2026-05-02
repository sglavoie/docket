package tui

import (
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
		if !row.IsSeparator {
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
