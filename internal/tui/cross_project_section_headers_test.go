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

	want := []string{models.SectionInbox, "Active", models.SectionSomeday}
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

func TestBuildCrossProjectRowsCustomSectionHeaders(t *testing.T) {
	m := minimalOutliner()

	customTask := &models.Task{ID: "ct1", Title: "Custom task", CustomSection: "Sprint-1"}
	activeTask := makeTask("at1", "Active task", "")

	groups := []projectTasksGroup{{
		projectID: "proj-1",
		tasks:     []*models.Task{customTask, activeTask},
	}}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	var headers []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			name := row.SectionName
			if name == "" {
				name = "Active"
			}
			headers = append(headers, name)
		}
	}

	want := []string{"Active", "Sprint-1"}
	if len(headers) != len(want) {
		t.Fatalf("expected headers %v, got %v", want, headers)
	}
	for i, w := range want {
		if headers[i] != w {
			t.Errorf("header[%d]: want %q, got %q", i, w, headers[i])
		}
	}
}

func TestBuildCrossProjectRowsCustomSectionPlacement(t *testing.T) {
	m := minimalOutliner()

	customTask := &models.Task{ID: "ct1", Title: "Custom task", CustomSection: "Sprint-1"}
	activeTask := makeTask("at1", "Active task", "")

	groups := []projectTasksGroup{{
		projectID: "proj-1",
		tasks:     []*models.Task{customTask, activeTask},
	}}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	currentSection := ""
	taskSections := make(map[string]string)
	for _, row := range m.rows {
		if row.IsSectionHeader {
			currentSection = row.SectionName
		} else if row.task != nil {
			taskSections[row.task.Title] = currentSection
		}
	}

	if got := taskSections["Custom task"]; got != "Sprint-1" {
		t.Errorf("Custom task should be under Sprint-1, got %q", got)
	}
	if got := taskSections["Active task"]; got != "" {
		t.Errorf("Active task should be under Active (empty), got %q", got)
	}
}

func TestBuildCrossProjectRowsMultipleCustomSectionsOrdered(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "p1",
			tasks: []*models.Task{
				{ID: "t1", Title: "T1", CustomSection: "Zulu"},
				{ID: "t3", Title: "T3", Section: models.SectionInbox},
			},
		},
		{
			projectID: "p2",
			tasks: []*models.Task{
				{ID: "t2", Title: "T2", CustomSection: "Alpha"},
			},
		},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	var headers []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row.SectionName)
		}
	}

	want := []string{models.SectionInbox, "Alpha", "Zulu"}
	if len(headers) != len(want) {
		t.Fatalf("expected headers %v, got %v", want, headers)
	}
	for i, w := range want {
		if headers[i] != w {
			t.Errorf("header[%d]: want %q, got %q", i, w, headers[i])
		}
	}
}
