package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// minimalOutliner returns a bare OutlinerModel suitable for unit tests that
// call buildCrossProjectRowsWithSectionHeaders, buildProjectGroupedRows, or
// flattenTaskTreeWithFoldsAndProjectID directly. It deliberately avoids
// config/storage so tests remain free of disk I/O.
func minimalOutliner() *OutlinerModel {
	return &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
	}
}

// makeTask creates a minimal task with the given title and section.
func makeTask(id, title, section string) *models.Task {
	return &models.Task{
		ID:      id,
		Title:   title,
		Section: section,
	}
}

// ── buildCrossProjectRowsWithSectionHeaders ──────────────────────────────────

// TestBuildCrossProjectRowsSectionOrdering verifies that section headers
// appear in canonical GTD order (Active/"", Inbox, Next, Waiting, Review,
// Standby, Someday) regardless of the order tasks are provided.
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

	// Collect section header names in the order they appear in m.rows.
	var headerOrder []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			name := row.SectionName
			if name == "" {
				name = "Active" // canonical display name for empty string
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

// TestBuildCrossProjectRowsEmptySectionSkipped verifies that sections with no
// tasks produce no header row.
func TestBuildCrossProjectRowsEmptySectionSkipped(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "proj-1",
			tasks: []*models.Task{
				makeTask("t1", "Next task", models.SectionNext),
			},
		},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	for _, row := range m.rows {
		if row.IsSectionHeader && row.SectionName != models.SectionNext {
			t.Errorf("unexpected section header %q (expected only Next)", row.SectionName)
		}
	}

	// Exactly one header expected.
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

// TestBuildCrossProjectRowsMultiProjectMerging verifies that tasks from two
// different projects in the same section each get a separate section header
// (with its own collapse key) and that every task row carries the correct
// ProjectID.
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

	// Both projects share the same section name, so there should be two
	// section headers — one per project.
	var headers []outlinerRow
	var taskRows []outlinerRow
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row)
		} else if !row.IsSeparator {
			taskRows = append(taskRows, row)
		}
	}

	if len(headers) != 2 {
		t.Errorf("expected 2 section headers for 2 projects, got %d", len(headers))
	}

	// Each header must have a composite collapse key and a non-empty ProjectID.
	for _, h := range headers {
		if h.ProjectID == "" {
			t.Error("section header has empty ProjectID")
		}
		expectedKey := h.ProjectID + ":" + h.SectionName
		if h.CollapseKey != expectedKey {
			t.Errorf("header CollapseKey: want %q, got %q", expectedKey, h.CollapseKey)
		}
	}

	// Task rows must carry their project ID.
	for _, row := range taskRows {
		if row.ProjectID == "" {
			t.Errorf("task row %q has empty ProjectID", row.task.Title)
		}
	}
}

// TestBuildCrossProjectRowsAllGroupsEmpty verifies that when the input groups
// slice is empty (or has no tasks), m.rows is not modified.
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

// ── buildProjectGroupedRows ───────────────────────────────────────────────────

// TestBuildProjectGroupedRowsSeparatorPlacement verifies that a single
// IsSeparator row is inserted between consecutive project groups, not before
// the first or after the last.
func TestBuildProjectGroupedRowsSeparatorPlacement(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
		{projectID: "p3", tasks: []*models.Task{makeTask("t3", "Task 3", "")}},
	}

	m.buildProjectGroupedRows(groups)

	// Count separators.
	separatorCount := 0
	for _, row := range m.rows {
		if row.IsSeparator {
			separatorCount++
		}
	}

	// With 3 groups, 2 separators expected (one between each pair).
	if separatorCount != 2 {
		t.Errorf("expected 2 separators for 3 groups, got %d", separatorCount)
	}

	// First row must not be a separator.
	if len(m.rows) == 0 {
		t.Fatal("no rows produced")
	}
	if m.rows[0].IsSeparator {
		t.Error("first row must not be a separator")
	}

	// Last row must not be a separator.
	if m.rows[len(m.rows)-1].IsSeparator {
		t.Error("last row must not be a separator")
	}
}

// TestBuildProjectGroupedRowsSingleGroup verifies that a single project group
// produces no separator rows.
func TestBuildProjectGroupedRowsSingleGroup(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
	}

	m.buildProjectGroupedRows(groups)

	for _, row := range m.rows {
		if row.IsSeparator {
			t.Error("single group must not produce any separator rows")
		}
	}
}

// TestBuildProjectGroupedRowsIndependentSorting verifies that each project's
// tasks appear as a contiguous block, preserving the insertion order supplied
// by the caller. Projects are NOT interleaved.
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
			tasks: []*models.Task{
				makeTask("b1", "Beta 1", ""),
			},
		},
	}

	m.buildProjectGroupedRows(groups)

	// Collect ProjectIDs in order, skipping separators.
	var ids []string
	for _, row := range m.rows {
		if !row.IsSeparator {
			ids = append(ids, row.ProjectID)
		}
	}

	// Expect: p1, p1, p2 — no interleaving.
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

// TestBuildProjectGroupedRowsEmptyGroups verifies that an empty groups slice
// leaves m.rows untouched.
func TestBuildProjectGroupedRowsEmptyGroups(t *testing.T) {
	m := minimalOutliner()

	m.buildProjectGroupedRows(nil)

	if len(m.rows) != 0 {
		t.Errorf("expected no rows for nil groups, got %d", len(m.rows))
	}
}

// ── separator skip loop ───────────────────────────────────────────────────────

// TestSeparatorSkipLoop verifies that separator rows are distinguishable from
// task rows and section headers via the IsSeparator field, matching what the
// render loop uses to skip them.
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
			// A separator row must have no task attached.
			if row.task != nil {
				t.Error("separator row must not carry a task")
			}
			// A separator must not also be a section header.
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

// TestSeparatorIsSeparatorNotSectionHeader verifies that separator rows set
// IsSeparator=true and IsSectionHeader=false, matching what the render and
// cursor-navigation loops use to distinguish them. The caller
// (crossProjectRebuildRows) pre-filters empty groups before passing to
// buildProjectGroupedRows, so all groups in production contain at least one
// task. This test mirrors that invariant by using non-empty groups only.
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
			t.Errorf("row[%d] is both IsSeparator and IsSectionHeader — must be mutually exclusive", i)
		}
		if row.IsSeparator && row.task != nil {
			t.Errorf("row[%d] is a separator but has a non-nil task", i)
		}
	}

	// Exactly 2 separators between 3 groups.
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

// TestSectionCountAccuracy verifies that the SectionCount on each header row
// reflects the recursive task count, including any nested children.
func TestSectionCountAccuracy(t *testing.T) {
	m := minimalOutliner()

	child := makeTask("c1", "Child task", models.SectionNext)
	parent := makeTask("p1", "Parent task", models.SectionNext)
	parent.Children = []*models.Task{child}

	groups := []projectTasksGroup{
		{projectID: "proj-1", tasks: []*models.Task{parent}},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	for _, row := range m.rows {
		if row.IsSectionHeader && row.SectionName == models.SectionNext {
			// parent + child = 2
			if row.SectionCount != 2 {
				t.Errorf("SectionCount: want 2, got %d", row.SectionCount)
			}
		}
	}
}
