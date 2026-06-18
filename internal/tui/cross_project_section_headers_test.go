package tui

import (
	"slices"
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

func TestBuildCrossProjectRowsNonGTDSectionsUseActiveHeader(t *testing.T) {
	m := minimalOutliner()

	legacyTask := &models.Task{ID: "ct1", Title: "Legacy task", Section: "Sprint-1"}
	activeTask := makeTask("at1", "Active task", "")

	groups := []projectTasksGroup{{
		projectID: "proj-1",
		tasks:     []*models.Task{legacyTask, activeTask},
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

	want := []string{"Active"}
	if len(headers) != len(want) {
		t.Fatalf("expected headers %v, got %v", want, headers)
	}
	for i, w := range want {
		if headers[i] != w {
			t.Errorf("header[%d]: want %q, got %q", i, w, headers[i])
		}
	}
}

func TestBuildCrossProjectRowsNonGTDSectionPlacementUnderActive(t *testing.T) {
	m := minimalOutliner()

	legacyTask := &models.Task{ID: "ct1", Title: "Legacy task", Section: "Sprint-1"}
	activeTask := makeTask("at1", "Active task", "")

	groups := []projectTasksGroup{{
		projectID: "proj-1",
		tasks:     []*models.Task{legacyTask, activeTask},
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

	if got := taskSections["Legacy task"]; got != "" {
		t.Errorf("Legacy task should be under Active (empty), got %q", got)
	}
	if got := taskSections["Active task"]; got != "" {
		t.Errorf("Active task should be under Active (empty), got %q", got)
	}
}

func TestBuildCrossProjectRowsMultipleNonGTDSectionsDoNotAddHeaders(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "p1",
			tasks: []*models.Task{
				{ID: "t1", Title: "T1"},
				{ID: "t3", Title: "T3", Section: models.SectionInbox},
			},
		},
		{
			projectID: "p2",
			tasks: []*models.Task{
				{ID: "t2", Title: "T2"},
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

	want := []string{models.SectionInbox, ""}
	if len(headers) != len(want) {
		t.Fatalf("expected headers %v, got %v", want, headers)
	}
	for i, w := range want {
		if headers[i] != w {
			t.Errorf("header[%d]: want %q, got %q", i, w, headers[i])
		}
	}
}

func TestBuildCrossProjectRowsLegacyNonGTDSectionValueStaysVisible(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{{
		projectID: "proj-1",
		tasks: []*models.Task{
			{ID: "legacy", Title: "Legacy task", Section: "Sprint-1"},
		},
	}}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	assertRowTitles(t, m.rows, []string{"[Active]", "Legacy task"})
}

func TestCrossProjectRebuildRowsSortsAllSharedSectionRowsByProjectName(t *testing.T) {
	alphaTask := makeTask("alpha-next", "Alpha next task", models.SectionNext)
	alphaTask.Status = models.StatusTodo
	alpha := &models.Project{
		ID:    "proj-alpha",
		Slug:  "alpha",
		Name:  "Alpha",
		Tasks: []*models.Task{alphaTask},
	}
	alpha.BuildTaskTree()

	betaTask := makeTask("beta-next", "Beta next task", models.SectionNext)
	betaTask.Status = models.StatusTodo
	beta := &models.Project{
		ID:    "proj-beta",
		Slug:  "beta",
		Name:  "Beta",
		Tasks: []*models.Task{betaTask},
	}
	beta.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{beta, alpha},
	}

	m.crossProjectRebuildRows()

	headerCount := 0
	var got []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			if row.SectionName == models.SectionNext {
				headerCount++
			}
			continue
		}
		if row.IsSeparator {
			t.Fatalf("All perspective should not emit project separators")
		}
		got = append(got, row.ProjectID)
	}

	if headerCount != 1 {
		t.Fatalf("expected one shared Next header, got %d", headerCount)
	}

	want := []string{"proj-alpha", "proj-beta"}
	if !slices.Equal(got, want) {
		t.Fatalf("shared section project order: want %v, got %v", want, got)
	}
}
