package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestColumnsForGroupBySection(t *testing.T) {
	defs := ColumnsForGroupBy(GroupBySection, nil)
	if len(defs) != 7 {
		t.Fatalf("expected 7 section columns, got %d", len(defs))
	}

	expectedKeys := []string{"", "Review", "Standby", "Next", "Inbox", "Waiting", "Someday"}
	for i, key := range expectedKeys {
		if defs[i].Key != key {
			t.Errorf("column %d: expected key %q, got %q", i, key, defs[i].Key)
		}
	}
}

func TestColumnsForGroupBySectionWithCustomSections(t *testing.T) {
	project := &models.Project{
		Sections: []string{"Next", "Inbox", "CustomA", "CustomB"},
	}
	// GroupBySection should NOT include custom sections
	defs := ColumnsForGroupBy(GroupBySection, project)
	if len(defs) != 7 {
		t.Fatalf("GroupBySection: expected 7 columns (no custom), got %d", len(defs))
	}

	// GroupBySectionAll SHOULD include custom sections
	defs = ColumnsForGroupBy(GroupBySectionAll, project)
	if len(defs) != 9 {
		t.Fatalf("GroupBySectionAll: expected 9 columns (7 GTD + 2 custom), got %d", len(defs))
	}
	if defs[7].Key != "CustomA" || defs[8].Key != "CustomB" {
		t.Errorf("custom sections not appended correctly: got %q and %q", defs[7].Key, defs[8].Key)
	}
}

func TestColumnsForGroupByStatus(t *testing.T) {
	defs := ColumnsForGroupBy(GroupByStatus, nil)
	if len(defs) != 7 {
		t.Fatalf("expected 7 status columns, got %d", len(defs))
	}
	expected := []string{"todo", "in_progress", "review", "standby", "blocked", "done", "canceled"}
	for i, key := range expected {
		if defs[i].Key != key {
			t.Errorf("column %d: expected key %q, got %q", i, key, defs[i].Key)
		}
	}
}

func TestColumnsForGroupByPriority(t *testing.T) {
	defs := ColumnsForGroupBy(GroupByPriority, nil)
	if len(defs) != 5 {
		t.Fatalf("expected 5 priority columns, got %d", len(defs))
	}
	expected := []string{"1", "2", "3", "4", "5"}
	for i, key := range expected {
		if defs[i].Key != key {
			t.Errorf("column %d: expected key %q, got %q", i, key, defs[i].Key)
		}
	}
}

func TestColumnsForGroupByType(t *testing.T) {
	defs := ColumnsForGroupBy(GroupByType, nil)
	if len(defs) != 5 {
		t.Fatalf("expected 5 type columns, got %d", len(defs))
	}
	expected := []string{"epic", "story", "task", "bug", "idea"}
	for i, key := range expected {
		if defs[i].Key != key {
			t.Errorf("column %d: expected key %q, got %q", i, key, defs[i].Key)
		}
	}
}

func TestTaskColumnKeySection(t *testing.T) {
	task := &models.Task{Section: "Next"}
	if key := TaskColumnKey(GroupBySection, task); key != "Next" {
		t.Errorf("expected %q, got %q", "Next", key)
	}
	task.Section = ""
	if key := TaskColumnKey(GroupBySection, task); key != "" {
		t.Errorf("expected empty string for Active, got %q", key)
	}
}

func TestTaskColumnKeyStatus(t *testing.T) {
	task := &models.Task{Status: models.StatusInProgress}
	if key := TaskColumnKey(GroupByStatus, task); key != "in_progress" {
		t.Errorf("expected %q, got %q", "in_progress", key)
	}
}

func TestTaskColumnKeyPriority(t *testing.T) {
	tests := []struct {
		priority int
		expected string
	}{
		{1, "1"},
		{2, "2"},
		{3, "3"},
		{4, "4"},
		{5, "5"},
		{6, "0"},  // >5 maps to "No Priority"
		{0, "0"},  // 0 maps to "No Priority"
		{-1, "0"}, // negative maps to "No Priority"
	}
	for _, tt := range tests {
		task := &models.Task{Priority: tt.priority}
		if key := TaskColumnKey(GroupByPriority, task); key != tt.expected {
			t.Errorf("priority %d: expected %q, got %q", tt.priority, tt.expected, key)
		}
	}
}

func TestTaskColumnKeyType(t *testing.T) {
	task := &models.Task{Type: models.TaskTypeBug}
	if key := TaskColumnKey(GroupByType, task); key != "bug" {
		t.Errorf("expected %q, got %q", "bug", key)
	}
}

func TestApplyColumnMoveSection(t *testing.T) {
	task := &models.Task{Section: ""}
	ApplyColumnMove(GroupBySection, task, "Next")
	if task.Section != "Next" {
		t.Errorf("expected section %q, got %q", "Next", task.Section)
	}
}

func TestApplyColumnMoveStatusToDone(t *testing.T) {
	task := &models.Task{Status: models.StatusInProgress}
	ApplyColumnMove(GroupByStatus, task, "done")
	if task.Status != models.StatusDone {
		t.Errorf("expected status done, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestApplyColumnMoveStatusToCanceled(t *testing.T) {
	task := &models.Task{Status: models.StatusTodo}
	ApplyColumnMove(GroupByStatus, task, "canceled")
	if task.Status != models.StatusCanceled {
		t.Errorf("expected status canceled, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestApplyColumnMoveStatusReopenFromDone(t *testing.T) {
	now := time.Now()
	task := &models.Task{Status: models.StatusDone, CompletedAt: &now}
	ApplyColumnMove(GroupByStatus, task, "todo")
	if task.Status != models.StatusTodo {
		t.Errorf("expected status todo, got %s", task.Status)
	}
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt to be cleared")
	}
}

func TestApplyColumnMoveStatusBetweenActive(t *testing.T) {
	task := &models.Task{Status: models.StatusTodo}
	ApplyColumnMove(GroupByStatus, task, "in_progress")
	if task.Status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", task.Status)
	}

	ApplyColumnMove(GroupByStatus, task, "blocked")
	if task.Status != models.StatusBlocked {
		t.Errorf("expected status blocked, got %s", task.Status)
	}

	ApplyColumnMove(GroupByStatus, task, "review")
	if task.Status != models.StatusReview {
		t.Errorf("expected status review, got %s", task.Status)
	}
}

func TestApplyColumnMovePriority(t *testing.T) {
	task := &models.Task{Priority: 3}
	ApplyColumnMove(GroupByPriority, task, "1")
	if task.Priority != 1 {
		t.Errorf("expected priority 1, got %d", task.Priority)
	}

	ApplyColumnMove(GroupByPriority, task, "0")
	if task.Priority != models.PriorityDefault {
		t.Errorf("expected default priority %d, got %d", models.PriorityDefault, task.Priority)
	}
}

func TestApplyColumnMoveType(t *testing.T) {
	task := &models.Task{Type: models.TaskTypeTask}
	ApplyColumnMove(GroupByType, task, "bug")
	if task.Type != models.TaskTypeBug {
		t.Errorf("expected type bug, got %s", task.Type)
	}
}

func TestGroupByLabel(t *testing.T) {
	tests := []struct {
		gb       KanbanGroupBy
		expected string
	}{
		{GroupBySection, "Section"},
		{GroupBySectionAll, "All Sections"},
		{GroupByStatus, "Status"},
		{GroupByPriority, "Priority"},
		{GroupByType, "Type"},
	}
	for _, tt := range tests {
		if label := GroupByLabel(tt.gb); label != tt.expected {
			t.Errorf("GroupByLabel(%d) = %q, want %q", tt.gb, label, tt.expected)
		}
	}
}

func TestIsRecentlyCompleted(t *testing.T) {
	now := time.Now()
	recent := now.Add(-3 * 24 * time.Hour)
	old := now.Add(-10 * 24 * time.Hour)

	task := &models.Task{CompletedAt: &recent}
	if !IsRecentlyCompleted(task, 7) {
		t.Error("expected task completed 3 days ago to be recent (7-day window)")
	}

	task.CompletedAt = &old
	if IsRecentlyCompleted(task, 7) {
		t.Error("expected task completed 10 days ago to not be recent (7-day window)")
	}

	task.CompletedAt = nil
	if IsRecentlyCompleted(task, 7) {
		t.Error("expected task with nil CompletedAt to not be recent")
	}
}

func TestIsCompletedStatusKey(t *testing.T) {
	if !IsCompletedStatusKey("done") {
		t.Error("expected 'done' to be a completed status key")
	}
	if !IsCompletedStatusKey("canceled") {
		t.Error("expected 'canceled' to be a completed status key")
	}
	if IsCompletedStatusKey("todo") {
		t.Error("expected 'todo' not to be a completed status key")
	}
}

func TestFindNextNonEmptyColumnWithStatusMode(t *testing.T) {
	makeTask := func() *models.Task {
		return &models.Task{Title: "test", Status: models.StatusTodo}
	}

	m := &KanbanViewModel{
		Project:      &models.Project{},
		groupBy:      GroupByStatus,
		columnDefs:   ColumnsForGroupBy(GroupByStatus, nil),
		Columns:      make(map[string]*ColumnState),
		Tasks:        make(map[string][]*models.Task),
		Nodes:        make(map[string][]*KanbanNode),
		ExpandState:  make(map[string]bool),
		ActiveColumn: 0,
	}

	// Populate: only "in_progress" column (index 1) has tasks
	for _, def := range m.columnDefs {
		m.Columns[def.Key] = &ColumnState{}
		m.Tasks[def.Key] = []*models.Task{}
		m.Nodes[def.Key] = []*KanbanNode{}
	}
	task := makeTask()
	m.Tasks["in_progress"] = []*models.Task{task}
	m.Nodes["in_progress"] = []*KanbanNode{{Task: task, Visible: true}}

	got := m.findNextNonEmptyColumn(1)
	if got != 1 {
		t.Errorf("expected column index 1 (in_progress), got %d", got)
	}
}

func TestColumnHelperMethods(t *testing.T) {
	m := &KanbanViewModel{
		groupBy:    GroupByStatus,
		columnDefs: ColumnsForGroupBy(GroupByStatus, nil),
	}

	keys := m.columnKeys()
	if len(keys) != 7 {
		t.Fatalf("expected 7 keys, got %d", len(keys))
	}
	if keys[0] != "todo" {
		t.Errorf("expected first key 'todo', got %q", keys[0])
	}

	m.ActiveColumn = 2
	if key := m.activeColumnKey(); key != "review" {
		t.Errorf("expected activeColumnKey 'review', got %q", key)
	}

	if idx := m.columnIndexForKey("blocked"); idx != 4 {
		t.Errorf("expected index 4 for 'blocked', got %d", idx)
	}

	if idx := m.columnIndexForKey("nonexistent"); idx != -1 {
		t.Errorf("expected -1 for nonexistent key, got %d", idx)
	}

	if label := m.columnLabel("in_progress"); label != "In Progress" {
		t.Errorf("expected label 'In Progress', got %q", label)
	}
}
