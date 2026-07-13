package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestTaskMatchesFieldFilters_Priority(t *testing.T) {
	m := &CommandPaletteModel{}
	task := &models.Task{Priority: 2}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "exact match",
			filters: []fieldFilter{{field: "priority", op: "eq", value: "2"}},
			want:    true,
		},
		{
			name:    "no match",
			filters: []fieldFilter{{field: "priority", op: "eq", value: "1"}},
			want:    false,
		},
		{
			name:    "range match",
			filters: []fieldFilter{{field: "priority", op: "range", value: "1-3"}},
			want:    true,
		},
		{
			name:    "range no match",
			filters: []fieldFilter{{field: "priority", op: "range", value: "3-5"}},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_Status(t *testing.T) {
	m := &CommandPaletteModel{}
	task := &models.Task{WorkflowState: models.StateInProgress}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "partial match",
			filters: []fieldFilter{{field: "status", op: "eq", value: "progress"}},
			want:    true,
		},
		{
			name:    "no match",
			filters: []fieldFilter{{field: "status", op: "eq", value: "done"}},
			want:    false,
		},
		{
			name:    "in match",
			filters: []fieldFilter{{field: "status", op: "in", alt: []string{"todo", "progress"}}},
			want:    true,
		},
		{
			name:    "in no match",
			filters: []fieldFilter{{field: "status", op: "in", alt: []string{"todo", "done"}}},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_Tag(t *testing.T) {
	m := &CommandPaletteModel{}
	task := &models.Task{Tags: []string{"bug", "urgent"}}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "exact match",
			filters: []fieldFilter{{field: "tag", op: "eq", value: "bug"}},
			want:    true,
		},
		{
			name:    "case insensitive",
			filters: []fieldFilter{{field: "tag", op: "eq", value: "BUG"}},
			want:    true,
		},
		{
			name:    "no match",
			filters: []fieldFilter{{field: "tag", op: "eq", value: "feature"}},
			want:    false,
		},
		{
			name:    "in match",
			filters: []fieldFilter{{field: "tag", op: "in", alt: []string{"feature", "urgent"}}},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_Section(t *testing.T) {
	m := &CommandPaletteModel{}
	// Section filters match the state-derived GTD bucket, not the raw legacy field.
	task := &models.Task{WorkflowState: models.StateNext}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "exact match",
			filters: []fieldFilter{{field: "section", op: "eq", value: "next"}},
			want:    true,
		},
		{
			name:    "case insensitive",
			filters: []fieldFilter{{field: "section", op: "eq", value: "NEXT"}},
			want:    true,
		},
		{
			name:    "no match",
			filters: []fieldFilter{{field: "section", op: "eq", value: "someday"}},
			want:    false,
		},
		{
			name:    "in match",
			filters: []fieldFilter{{field: "section", op: "in", alt: []string{"someday", "next"}}},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_SectionGTDOnly(t *testing.T) {
	m := &CommandPaletteModel{}
	task := &models.Task{WorkflowState: models.StateInbox}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "no match by non-GTD section eq",
			filters: []fieldFilter{{field: "section", op: "eq", value: "Sprint1"}},
			want:    false,
		},
		{
			name:    "match by GTD Section eq",
			filters: []fieldFilter{{field: "section", op: "eq", value: models.SectionInbox}},
			want:    true,
		},
		{
			name:    "no match wrong section",
			filters: []fieldFilter{{field: "section", op: "eq", value: "Sprint2"}},
			want:    false,
		},
		{
			name:    "in match via GTD Section",
			filters: []fieldFilter{{field: "section", op: "in", alt: []string{"Sprint1", models.SectionInbox}}},
			want:    true,
		},
		{
			name:    "in no match",
			filters: []fieldFilter{{field: "section", op: "in", alt: []string{"Sprint2", "Sprint3"}}},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildSectionListIgnoresNonGTDSections(t *testing.T) {
	project, err := models.NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Sections = []string{models.SectionNext, models.SectionInbox, "Sprint1"}

	// Non-GTD custom sections are no longer representable on a task; the task
	// state buckets to a GTD section. The non-GTD exclusion is still exercised
	// via project.Sections above ("Sprint1"), which buildSectionList drops.
	task1, _ := models.NewTask("1", 1, models.TaskTypeTask, "Task 1")
	task1.SetSection(models.SectionInbox)

	task2, _ := models.NewTask("2", 2, models.TaskTypeTask, "Task 2")
	task2.SetSection(models.SectionInbox)

	project.Tasks = []*models.Task{task1, task2}

	sections := buildSectionList(project)

	foundNonGTD := false
	foundInbox := false
	for _, s := range sections {
		if s.Name == "Sprint1" {
			foundNonGTD = true
		}
		if s.Name == models.SectionInbox {
			foundInbox = true
		}
	}
	if foundNonGTD {
		t.Error("Sprint1 section should not appear in buildSectionList output")
	}
	if !foundInbox {
		t.Error("Inbox section not found in buildSectionList output")
	}
}

func TestTaskMatchesFieldFilters_Date(t *testing.T) {
	m := &CommandPaletteModel{}
	now := time.Now()
	today := util.StartOfDay(now)

	tests := []struct {
		name    string
		dueDate *time.Time
		filter  string
		want    bool
	}{
		{
			name:    "today match",
			dueDate: &today,
			filter:  "today",
			want:    true,
		},
		{
			name:    "overdue",
			dueDate: func() *time.Time { d := today.AddDate(0, 0, -1); return &d }(),
			filter:  "overdue",
			want:    true,
		},
		{
			name:    "not overdue",
			dueDate: &today,
			filter:  "overdue",
			want:    false,
		},
		{
			name:    "tomorrow",
			dueDate: func() *time.Time { d := today.AddDate(0, 0, 1); return &d }(),
			filter:  "tomorrow",
			want:    true,
		},
		{
			name:    "this week",
			dueDate: func() *time.Time { d := today.AddDate(0, 0, 3); return &d }(),
			filter:  "this-week",
			want:    true,
		},
		{
			name:    "no due date",
			dueDate: nil,
			filter:  "today",
			want:    false,
		},
		{
			name:    "none matches nil due date",
			dueDate: nil,
			filter:  "none",
			want:    true,
		},
		{
			name:    "none rejects task with due date",
			dueDate: &today,
			filter:  "none",
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{DueDate: tc.dueDate}
			item := PaletteItem{Category: ItemCategoryTasks, Task: task}
			filters := []fieldFilter{{field: "date", op: "eq", value: tc.filter}}

			got := m.taskMatchesFieldFilters(item, filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_OverdueExcludesCompleted(t *testing.T) {
	m := &CommandPaletteModel{}
	now := time.Now()
	today := util.StartOfDay(now)
	yesterday := today.AddDate(0, 0, -1)

	task := &models.Task{
		DueDate:       &yesterday,
		WorkflowState: models.StateDone,
	}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}
	filters := []fieldFilter{{field: "date", op: "eq", value: "overdue"}}

	got := m.taskMatchesFieldFilters(item, filters)
	if got {
		t.Error("completed task with past due date should not match @overdue filter")
	}

	task.WorkflowState = models.StateCanceled
	got = m.taskMatchesFieldFilters(item, filters)
	if got {
		t.Error("canceled task with past due date should not match @overdue filter")
	}

	task.WorkflowState = models.StateReady
	got = m.taskMatchesFieldFilters(item, filters)
	if !got {
		t.Error("active task with past due date should match @overdue filter")
	}
}

func TestTaskMatchesFieldFilters_NonTaskItem(t *testing.T) {
	m := &CommandPaletteModel{}
	item := PaletteItem{Category: ItemCategoryCommands, Command: &Command{Name: "test"}}

	got := m.taskMatchesFieldFilters(item, nil)
	if !got {
		t.Error("expected non-task item with no filters to match")
	}

	got = m.taskMatchesFieldFilters(item, []fieldFilter{{field: "priority", op: "eq", value: "1"}})
	if got {
		t.Error("expected non-task item with filters to not match")
	}
}
