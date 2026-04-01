package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildCrossProjectRowsSectionOrdering(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "proj-1",
			tasks: []*models.Task{
				makeTask("t1", "Someday task", models.SectionSomeday),
				makeTask("t2", "Inbox task", models.SectionInbox),
				makeTask("t3", "Active task", ""),
			},
		},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	var headerOrder []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			name := row.SectionName
			if name == "" {
				name = "Active"
			}
			headerOrder = append(headerOrder, name)
		}
	}

	want := []string{"Active", models.SectionInbox, models.SectionSomeday}
	if len(headerOrder) != len(want) {
		t.Fatalf("expected %d section headers, got %d: %v", len(want), len(headerOrder), headerOrder)
	}
	for i, name := range want {
		if headerOrder[i] != name {
			t.Errorf("header[%d]: want %q, got %q", i, name, headerOrder[i])
		}
	}
}

func TestBuildCrossProjectRowsEmptySectionSkipped(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{{
		projectID: "proj-1",
		tasks:     []*models.Task{makeTask("t1", "Next task", models.SectionNext)},
	}}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	for _, row := range m.rows {
		if row.IsSectionHeader && row.SectionName != models.SectionNext {
			t.Errorf("unexpected section header %q (expected only Next)", row.SectionName)
		}
	}

	headerCount := 0
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headerCount++
		}
	}
	if headerCount != 1 {
		t.Errorf("expected 1 section header, got %d", headerCount)
	}
}

func TestBuildCrossProjectRowsMultiProjectMerging(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "proj-a",
			tasks:     []*models.Task{makeTask("ta1", "A task", models.SectionNext)},
		},
		{
			projectID: "proj-b",
			tasks:     []*models.Task{makeTask("tb1", "B task", models.SectionNext)},
		},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	var headers []outlinerRow
	var taskRows []outlinerRow
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row)
		} else if !row.IsSeparator {
			taskRows = append(taskRows, row)
		}
	}

	if len(headers) != 1 {
		t.Errorf("expected 1 shared section header for 2 projects in same section, got %d", len(headers))
	}

	if len(headers) == 1 && headers[0].ProjectID != "" {
		t.Errorf("shared section header should have empty ProjectID, got %q", headers[0].ProjectID)
	}

	wantCollapseKey := "_xp:" + models.SectionNext
	if len(headers) == 1 && headers[0].CollapseKey != wantCollapseKey {
		t.Errorf("shared header CollapseKey: want %q, got %q", wantCollapseKey, headers[0].CollapseKey)
	}

	if len(headers) == 1 && headers[0].SectionCount != 2 {
		t.Errorf("shared header SectionCount: want 2, got %d", headers[0].SectionCount)
	}

	projectIDs := make(map[string]bool)
	for _, row := range taskRows {
		if row.ProjectID == "" {
			t.Errorf("task row %q has empty ProjectID", row.task.Title)
		}
		projectIDs[row.ProjectID] = true
	}
	if !projectIDs["proj-a"] || !projectIDs["proj-b"] {
		t.Error("expected task rows from both proj-a and proj-b")
	}
}

func TestBuildCrossProjectRowsAllGroupsEmpty(t *testing.T) {
	m := minimalOutliner()

	m.buildCrossProjectRowsWithSectionHeaders(nil)
	if len(m.rows) != 0 {
		t.Errorf("expected empty rows for nil groups, got %d rows", len(m.rows))
	}

	m.buildCrossProjectRowsWithSectionHeaders([]projectTasksGroup{})
	if len(m.rows) != 0 {
		t.Errorf("expected empty rows for empty groups, got %d rows", len(m.rows))
	}
}

func TestBuildProjectGroupedRowsSeparatorPlacement(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
		{projectID: "p3", tasks: []*models.Task{makeTask("t3", "Task 3", "")}},
	}

	m.buildProjectGroupedRows(groups)

	separatorCount := 0
	for _, row := range m.rows {
		if row.IsSeparator {
			separatorCount++
		}
	}

	if separatorCount != 2 {
		t.Errorf("expected 2 separators for 3 groups, got %d", separatorCount)
	}
	if len(m.rows) == 0 {
		t.Fatal("no rows produced")
	}
	if m.rows[0].IsSeparator {
		t.Error("first row must not be a separator")
	}
	if m.rows[len(m.rows)-1].IsSeparator {
		t.Error("last row must not be a separator")
	}
}

func TestBuildProjectGroupedRowsSingleGroup(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{{
		projectID: "p1",
		tasks:     []*models.Task{makeTask("t1", "Task 1", "")},
	}}

	m.buildProjectGroupedRows(groups)

	for _, row := range m.rows {
		if row.IsSeparator {
			t.Error("single group must not produce any separator rows")
		}
	}
}

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

func TestSeparatorSkipLoop(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task A", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task B", "")}},
	}

	m.buildProjectGroupedRows(groups)

	taskRowCount := 0
	separatorCount := 0
	for _, row := range m.rows {
		switch {
		case row.IsSeparator:
			separatorCount++
			if row.task != nil {
				t.Error("separator row must not carry a task")
			}
			if row.IsSectionHeader {
				t.Error("separator row must not also be IsSectionHeader")
			}
		case !row.IsSectionHeader:
			taskRowCount++
		}
	}

	if separatorCount != 1 {
		t.Errorf("expected 1 separator between 2 groups, got %d", separatorCount)
	}
	if taskRowCount != 2 {
		t.Errorf("expected 2 task rows, got %d", taskRowCount)
	}
}

func TestSeparatorIsSeparatorNotSectionHeader(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
		{projectID: "p3", tasks: []*models.Task{makeTask("t3", "Task 3", "")}},
	}

	m.buildProjectGroupedRows(groups)

	for i, row := range m.rows {
		if row.IsSeparator && row.IsSectionHeader {
			t.Errorf("row[%d] is both IsSeparator and IsSectionHeader - must be mutually exclusive", i)
		}
		if row.IsSeparator && row.task != nil {
			t.Errorf("row[%d] is a separator but has a non-nil task", i)
		}
	}

	separatorCount := 0
	for _, row := range m.rows {
		if row.IsSeparator {
			separatorCount++
		}
	}
	if separatorCount != 2 {
		t.Errorf("expected 2 separators, got %d", separatorCount)
	}
}

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