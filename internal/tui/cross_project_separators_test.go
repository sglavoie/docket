package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildProjectGroupedRowsHeaderPlacement(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
		{projectID: "p3", tasks: []*models.Task{makeTask("t3", "Task 3", "")}},
	}

	m.buildProjectGroupedRows(groups)

	separatorCount := 0
	projectHeaderCount := 0
	for _, row := range m.rows {
		if row.IsSeparator {
			separatorCount++
		}
		if row.IsProjectHeader {
			projectHeaderCount++
		}
	}

	if separatorCount != 0 {
		t.Errorf("expected 0 separators for 3 groups, got %d", separatorCount)
	}
	if projectHeaderCount != 3 {
		t.Errorf("expected 3 project headers for 3 groups, got %d", projectHeaderCount)
	}
	if len(m.rows) == 0 {
		t.Fatal("no rows produced")
	}
	if !m.rows[0].IsProjectHeader {
		t.Error("first row must be a project header")
	}
}

func TestBuildProjectGroupedRowsSingleGroup(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{{
		projectID: "p1",
		tasks:     []*models.Task{makeTask("t1", "Task 1", "")},
	}}

	m.buildProjectGroupedRows(groups)

	projectHeaderCount := 0
	for _, row := range m.rows {
		if row.IsSeparator {
			t.Error("single group must not produce any separator rows")
		}
		if row.IsProjectHeader {
			projectHeaderCount++
		}
	}
	if projectHeaderCount != 1 {
		t.Errorf("expected exactly 1 project header for a single group, got %d", projectHeaderCount)
	}
}

func TestProjectHeaderSkipLoop(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task A", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task B", "")}},
	}

	m.buildProjectGroupedRows(groups)

	taskRowCount := 0
	projectHeaderCount := 0
	for _, row := range m.rows {
		switch {
		case row.IsProjectHeader:
			projectHeaderCount++
			if row.task != nil {
				t.Error("project header row must not carry a task")
			}
			if row.IsSectionHeader {
				t.Error("project header row must not also be IsSectionHeader")
			}
			if row.IsSeparator {
				t.Error("project header row must not also be IsSeparator")
			}
		case row.isTaskRow():
			taskRowCount++
		}
	}

	if projectHeaderCount != 2 {
		t.Errorf("expected 2 project headers for 2 groups, got %d", projectHeaderCount)
	}
	if taskRowCount != 2 {
		t.Errorf("expected 2 task rows, got %d", taskRowCount)
	}
}

// TestSpaceOnProjectHeaderNoPanic verifies that the onHeader guard correctly
// treats project header rows as non-task rows so the multi-select space prefix
// cannot deref the nil task on a project header.
func TestSpaceOnProjectHeaderNoPanic(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
	}

	m.buildProjectGroupedRows(groups)

	m.multiSelectMode = true
	m.selectedTaskIdentities = make(map[TaskIdentity]bool)

	headerIdx := -1
	for i, row := range m.rows {
		if row.IsProjectHeader {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatal("expected a project header row")
	}

	m.cursor = headerIdx

	row := m.rows[m.cursor]
	if row.task != nil {
		t.Error("project header row should carry no task payload")
	}
	if row.isTaskRow() {
		t.Error("project header row should not be classified as a task row")
	}
	// Project headers are navigable (Enter scopes, Shift+Enter administers); the
	// space multi-select guard is safe because it keys off isTaskRow, not
	// isNavigable.
	if !row.isNavigable() {
		t.Error("project header row should be navigable")
	}
}

// TestDeleteOnProjectHeaderNoPanic verifies that delete-key guards relying on
// isNavigable / isTaskRow correctly route around project header rows so D
// cannot deref the nil task on a project header.
func TestDeleteOnProjectHeaderNoPanic(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
	}

	m.buildProjectGroupedRows(groups)

	for i, row := range m.rows {
		if row.IsProjectHeader {
			m.cursor = i
			cur := m.rows[m.cursor]
			if cur.isTaskRow() {
				t.Error("project header row must not be classified as a task row")
			}
			// Delete safety comes from the isTaskRow/nil-task guards, not from
			// non-navigability: headers are navigable so Enter/Shift+Enter work.
			if !cur.isNavigable() {
				t.Error("project header row must be navigable")
			}
			if cur.task != nil {
				t.Error("project header row must carry no task payload")
			}
			return
		}
	}
	t.Fatal("expected a project header row")
}

func TestProjectHeaderIsProjectHeaderNotSectionHeader(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
		{projectID: "p3", tasks: []*models.Task{makeTask("t3", "Task 3", "")}},
	}

	m.buildProjectGroupedRows(groups)

	for i, row := range m.rows {
		if row.IsProjectHeader && row.IsSectionHeader {
			t.Errorf("row[%d] is both IsProjectHeader and IsSectionHeader - must be mutually exclusive", i)
		}
		if row.IsProjectHeader && row.IsSeparator {
			t.Errorf("row[%d] is both IsProjectHeader and IsSeparator - must be mutually exclusive", i)
		}
		if row.IsProjectHeader && row.task != nil {
			t.Errorf("row[%d] is a project header but has a non-nil task", i)
		}
	}

	projectHeaderCount := 0
	for _, row := range m.rows {
		if row.IsProjectHeader {
			projectHeaderCount++
		}
	}
	if projectHeaderCount != 3 {
		t.Errorf("expected 3 project headers, got %d", projectHeaderCount)
	}
}
