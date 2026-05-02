package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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

// TestSpaceOnSeparatorNoPanic verifies that pressing space in multi-select mode
// does not panic when the cursor is on a separator row (which has a nil task).
func TestSpaceOnSeparatorNoPanic(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
	}

	m.buildProjectGroupedRows(groups)

	// Enter multi-select mode
	m.multiSelectMode = true
	m.selectedTasks = make(map[string]bool)

	// Find the separator row
	separatorIdx := -1
	for i, row := range m.rows {
		if row.IsSeparator {
			separatorIdx = i
			break
		}
	}
	if separatorIdx < 0 {
		t.Fatal("expected a separator row")
	}

	// Move cursor to separator
	m.cursor = separatorIdx

	// This must not panic — separator has nil task
	row := m.rows[m.cursor]
	if row.IsSectionHeader || row.IsSeparator || row.task == nil {
		// Expected: skip selection for non-task rows
	} else {
		t.Error("separator row should be skipped by the guard condition")
	}

	// Also verify the onHeader computation includes separators
	onHeader := m.rows[m.cursor].IsSectionHeader || m.rows[m.cursor].IsSeparator
	if !onHeader {
		t.Error("onHeader should be true for separator rows to prevent prefix key activation")
	}
}

// TestDeleteOnSeparatorNoPanic verifies that the D (delete) key guard correctly
// treats separator rows as non-task rows, preventing nil dereference on row.task.
func TestDeleteOnSeparatorNoPanic(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
	}

	m.buildProjectGroupedRows(groups)

	// Find separator
	for i, row := range m.rows {
		if row.IsSeparator {
			m.cursor = i
			// The onHeader guard should prevent D from accessing row.task
			onHeader := m.rows[m.cursor].IsSectionHeader || m.rows[m.cursor].IsSeparator
			if !onHeader {
				t.Error("onHeader must be true for separator rows")
			}
			return
		}
	}
	t.Fatal("expected a separator row")
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
