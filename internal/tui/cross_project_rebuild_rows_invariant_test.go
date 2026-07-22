package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectRebuildRowsProjectIDInvariant(t *testing.T) {
	projAlpha := &models.Project{
		ID:   "proj-alpha",
		Name: "Alpha",
		Tasks: []*models.Task{
			makeTask("ta1", "Alpha task 1", ""),
			makeTask("ta2", "Alpha task 2", models.SectionNext),
		},
	}
	projAlpha.BuildTaskTree()

	projBeta := &models.Project{
		ID:   "proj-beta",
		Name: "Beta",
		Tasks: []*models.Task{
			makeTask("tb1", "Beta task 1", models.SectionInbox),
			makeTask("tb2", "Beta task 2", models.SectionNext),
		},
	}
	projBeta.BuildTaskTree()

	taskOwner := make(map[string]string)
	for _, proj := range []*models.Project{projAlpha, projBeta} {
		for _, task := range proj.Tasks {
			taskOwner[task.ID] = proj.ID
		}
	}

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		activePerspective: 1,
		loadedProjects:    []*models.Project{projAlpha, projBeta},
	}

	m.crossProjectRebuildRows()

	if len(m.rows) == 0 {
		t.Fatal("crossProjectRebuildRows produced no rows")
	}

	for i, row := range m.rows {
		switch {
		case row.IsSectionHeader:
			if row.ProjectID != "" {
				t.Errorf("row[%d] IsSectionHeader but has non-empty ProjectID %q", i, row.ProjectID)
			}
		case row.IsProjectHeader:
			if row.ProjectID == "" {
				t.Errorf("row[%d] IsProjectHeader but has empty ProjectID", i)
			}
			if row.task != nil {
				t.Errorf("row[%d] IsProjectHeader but carries task payload %q", i, row.task.Title)
			}
		case row.IsSeparator:
			if row.ProjectID != "" {
				t.Errorf("row[%d] IsSeparator but has non-empty ProjectID %q", i, row.ProjectID)
			}
		default:
			if row.ProjectID == "" {
				title := ""
				if row.task != nil {
					title = row.task.Title
				}
				t.Errorf("row[%d] task row %q has empty ProjectID", i, title)
			}
			if row.task != nil {
				wantProjectID := taskOwner[row.task.ID]
				if wantProjectID != "" && row.ProjectID != wantProjectID {
					t.Errorf("row[%d] task %q: ProjectID want %q, got %q", i, row.task.Title, wantProjectID, row.ProjectID)
				}
			}
		}
	}
}
