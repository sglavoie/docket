package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSectionCountAccuracy(t *testing.T) {
	m := minimalOutliner()

	child := makeTask("c1", "Child task", models.SectionNext)
	parent := makeTask("p1", "Parent task", models.SectionNext)
	parent.Children = []*models.Task{child}

	groups := []projectTasksGroup{{
		projectID: "proj-1",
		tasks:     []*models.Task{parent},
	}}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	for _, row := range m.rows {
		if row.IsSectionHeader && row.SectionName == models.SectionNext && row.SectionCount != 2 {
			t.Errorf("SectionCount: want 2, got %d", row.SectionCount)
		}
	}
}

//nolint:gocyclo // table-driven invariant exercises many code paths in one test
func TestProjectIDPopulationInvariant(t *testing.T) {
	groups := []projectTasksGroup{
		{
			projectID: "proj-alpha",
			tasks: []*models.Task{
				makeTask("ta1", "Alpha active", ""),
				makeTask("ta2", "Alpha next", models.SectionNext),
			},
		},
		{
			projectID: "proj-beta",
			tasks: []*models.Task{
				makeTask("tb1", "Beta next", models.SectionNext),
				makeTask("tb2", "Beta inbox", models.SectionInbox),
			},
		},
	}

	t.Run("buildCrossProjectRowsWithSectionHeaders", func(t *testing.T) {
		m := minimalOutliner()
		m.buildCrossProjectRowsWithSectionHeaders(groups)

		if len(m.rows) == 0 {
			t.Fatal("expected non-empty rows")
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
				var resolvable bool
				for _, g := range groups {
					if g.projectID == row.ProjectID {
						resolvable = true
						break
					}
				}
				if !resolvable {
					t.Errorf("row[%d] IsProjectHeader ProjectID %q does not match any input group", i, row.ProjectID)
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
					var wantProjectID string
					for _, g := range groups {
						for _, task := range g.tasks {
							if task.ID == row.task.ID {
								wantProjectID = g.projectID
							}
						}
					}
					if wantProjectID != "" && row.ProjectID != wantProjectID {
						t.Errorf("row[%d] task %q: ProjectID want %q, got %q", i, row.task.Title, wantProjectID, row.ProjectID)
					}
				}
			}
		}
	})

	t.Run("buildProjectGroupedRows", func(t *testing.T) {
		m := minimalOutliner()
		m.buildProjectGroupedRows(groups)

		if len(m.rows) == 0 {
			t.Fatal("expected non-empty rows")
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
				var resolvable bool
				for _, g := range groups {
					if g.projectID == row.ProjectID {
						resolvable = true
						break
					}
				}
				if !resolvable {
					t.Errorf("row[%d] IsProjectHeader ProjectID %q does not match any input group", i, row.ProjectID)
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
					var wantProjectID string
					for _, g := range groups {
						for _, task := range g.tasks {
							if task.ID == row.task.ID {
								wantProjectID = g.projectID
							}
						}
					}
					if wantProjectID != "" && row.ProjectID != wantProjectID {
						t.Errorf("row[%d] task %q: ProjectID want %q, got %q", i, row.task.Title, wantProjectID, row.ProjectID)
					}
				}
			}
		}
	})
}
