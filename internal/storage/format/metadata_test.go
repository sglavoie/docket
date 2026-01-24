package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestParseMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metaStr  string
		want     *models.Task
		warnings int
	}{
		{
			name:    "basic task metadata",
			metaStr: "id: task-123, status: todo, priority: 3",
			want: &models.Task{
				ID:       "task-123",
				Status:   models.StatusTodo,
				Priority: 3,
			},
			warnings: 0,
		},
		{
			name:    "complete task metadata",
			metaStr: "id: task-456, status: done, priority: 1, tags: work;urgent, section: Backend",
			want: &models.Task{
				ID:       "task-456",
				Status:   models.StatusDone,
				Priority: 1,
				Tags:     []string{"work", "urgent"},
				Section:  "Backend",
			},
			warnings: 0,
		},
		{
			name:    "task with dates",
			metaStr: "id: t1, status: todo, priority: 2, created: 2025-01-15 10:30, updated: 2025-01-16 14:20, due: 2025-02-01",
			want: &models.Task{
				ID:        "t1",
				Status:    models.StatusTodo,
				Priority:  2,
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt: time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
				DueDate:   timePtr(time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)),
			},
			warnings: 0,
		},
		{
			name:    "task with completed timestamp",
			metaStr: "id: t2, status: done, priority: 1, completed: 2025-01-20 09:15",
			want: &models.Task{
				ID:          "t2",
				Status:      models.StatusDone,
				Priority:    1,
				CompletedAt: timePtr(time.Date(2025, 1, 20, 9, 15, 0, 0, time.Local)),
			},
			warnings: 0,
		},
		{
			name:    "task with dependencies",
			metaStr: "id: t3, status: blocked, priority: 2, depends: task-1;task-2;task-3",
			want: &models.Task{
				ID:        "t3",
				Status:    models.StatusBlocked,
				Priority:  2,
				DependsOn: []string{"task-1", "task-2", "task-3"},
			},
			warnings: 0,
		},
		{
			name:    "task with parent",
			metaStr: "id: subtask-1, status: todo, priority: 3, parent: parent-task-1",
			want: &models.Task{
				ID:       "subtask-1",
				Status:   models.StatusTodo,
				Priority: 3,
				ParentID: strPtr("parent-task-1"),
			},
			warnings: 0,
		},
		{
			name:    "task with boolean flags",
			metaStr: "id: t4, status: todo, priority: 2, pinned: true, active: true",
			want: &models.Task{
				ID:       "t4",
				Status:   models.StatusTodo,
				Priority: 2,
				Pinned:   true,
				Active:   true,
			},
			warnings: 0,
		},
		{
			name:    "task with simple recurrence",
			metaStr: "id: t5, status: todo, priority: 1, recur: 1d",
			want: &models.Task{
				ID:       "t5",
				Status:   models.StatusTodo,
				Priority: 1,
				Recurrence: &models.RecurrencePattern{
					Interval: 1,
					Unit:     models.RecurrenceDaily,
				},
			},
			warnings: 0,
		},
		{
			name:    "task with weekly recurrence and weekdays",
			metaStr: "id: t6, status: todo, priority: 2, recur: 1w:mon,wed,fri",
			want: &models.Task{
				ID:       "t6",
				Status:   models.StatusTodo,
				Priority: 2,
				Recurrence: &models.RecurrencePattern{
					Interval: 1,
					Unit:     models.RecurrenceWeekly,
					Weekdays: []int{1, 3, 5},
				},
			},
			warnings: 0,
		},
		{
			name:    "invalid priority - out of range",
			metaStr: "id: t7, status: todo, priority: 10",
			want: &models.Task{
				ID:       "t7",
				Status:   models.StatusTodo,
				Priority: 0,
			},
			warnings: 1,
		},
		{
			name:    "invalid priority - non-numeric",
			metaStr: "id: t8, status: todo, priority: high",
			want: &models.Task{
				ID:       "t8",
				Status:   models.StatusTodo,
				Priority: 0,
			},
			warnings: 1,
		},
		{
			name:    "empty tags and dependencies are handled",
			metaStr: "id: t9, status: todo, priority: 1, tags: , depends: ",
			want: &models.Task{
				ID:       "t9",
				Status:   models.StatusTodo,
				Priority: 1,
			},
			warnings: 0,
		},
		{
			name:    "whitespace handling in tags and dependencies",
			metaStr: "id: t10, status: todo, priority: 1, tags:  work ; urgent  , depends:  dep1 ; dep2  ",
			want: &models.Task{
				ID:        "t10",
				Status:    models.StatusTodo,
				Priority:  1,
				Tags:      []string{"work", "urgent"},
				DependsOn: []string{"dep1", "dep2"},
			},
			warnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{}
			warnings := ParseMetadata(task, tt.metaStr)

			if len(warnings) != tt.warnings {
				t.Errorf("got %d warnings, want %d", len(warnings), tt.warnings)
			}

			assertTaskFields(t, task, tt.want)
		})
	}
}

func TestParseMetadata_RecurrenceWithCommas(t *testing.T) {
	metaStr := "id: t1, status: todo, priority: 2, recur: 2w:mon,wed,fri, tags: work"
	task := &models.Task{}
	warnings := ParseMetadata(task, metaStr)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0", len(warnings))
	}

	if task.ID != "t1" {
		t.Errorf("ID = %q, want %q", task.ID, "t1")
	}

	if task.Recurrence == nil {
		t.Fatal("Recurrence is nil")
	}

	if task.Recurrence.Interval != 2 {
		t.Errorf("Recurrence.Interval = %d, want 2", task.Recurrence.Interval)
	}

	if task.Recurrence.Unit != models.RecurrenceWeekly {
		t.Errorf("Recurrence.Unit = %s, want %s", task.Recurrence.Unit, models.RecurrenceWeekly)
	}

	wantWeekdays := []int{1, 3, 5}
	if !intSlicesEqual(task.Recurrence.Weekdays, wantWeekdays) {
		t.Errorf("Recurrence.Weekdays = %v, want %v", task.Recurrence.Weekdays, wantWeekdays)
	}

	if !stringSlicesEqual(task.Tags, []string{"work"}) {
		t.Errorf("Tags = %v, want [work]", task.Tags)
	}
}

func TestWriteMetadata(t *testing.T) {
	tests := []struct {
		name string
		task *models.Task
		want string
	}{
		{
			name: "basic task",
			task: &models.Task{
				ID:       "task-123",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 3,
			},
			want: "<!-- id: task-123, status: todo, priority: 3 -->",
		},
		{
			name: "task with tags",
			task: &models.Task{
				ID:       "task-456",
				Type:     models.TaskTypeTask,
				Status:   models.StatusDone,
				Priority: 1,
				Tags:     []string{"work", "urgent"},
			},
			want: "<!-- id: task-456, status: done, priority: 1, tags: work;urgent -->",
		},
		{
			name: "task with section",
			task: &models.Task{
				ID:       "task-789",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				Section:  "Backend",
			},
			want: "<!-- id: task-789, status: todo, priority: 2, section: Backend -->",
		},
		{
			name: "task with dates",
			task: &models.Task{
				ID:        "task-1",
				Type:      models.TaskTypeTask,
				Status:    models.StatusTodo,
				Priority:  2,
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt: time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
				DueDate:   timePtr(time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)),
			},
			want: "<!-- id: task-1, status: todo, priority: 2, created: 2025-01-15 10:30, updated: 2025-01-16 14:20, due: 2025-02-01 -->",
		},
		{
			name: "task with completed timestamp",
			task: &models.Task{
				ID:          "task-2",
				Type:        models.TaskTypeTask,
				Status:      models.StatusDone,
				Priority:    1,
				CreatedAt:   time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				CompletedAt: timePtr(time.Date(2025, 1, 20, 9, 15, 0, 0, time.Local)),
			},
			want: "<!-- id: task-2, status: done, priority: 1, created: 2025-01-15 10:30, completed: 2025-01-20 09:15 -->",
		},
		{
			name: "task with dependencies",
			task: &models.Task{
				ID:        "task-3",
				Type:      models.TaskTypeTask,
				Status:    models.StatusBlocked,
				Priority:  2,
				DependsOn: []string{"task-1", "task-2"},
			},
			want: "<!-- id: task-3, status: blocked, priority: 2, depends: task-1;task-2 -->",
		},
		{
			name: "task with boolean flags",
			task: &models.Task{
				ID:       "task-4",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				Pinned:   true,
				Active:   true,
			},
			want: "<!-- id: task-4, status: todo, priority: 2, pinned: true, active: true -->",
		},
		{
			name: "task with daily recurrence",
			task: &models.Task{
				ID:       "task-5",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 1,
				Recurrence: &models.RecurrencePattern{
					Interval: 1,
					Unit:     models.RecurrenceDaily,
				},
			},
			want: "<!-- id: task-5, status: todo, priority: 1, recur: 1d -->",
		},
		{
			name: "task with weekly recurrence and weekdays",
			task: &models.Task{
				ID:       "task-6",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				Recurrence: &models.RecurrencePattern{
					Interval: 2,
					Unit:     models.RecurrenceWeekly,
					Weekdays: []int{1, 3, 5},
				},
			},
			want: "<!-- id: task-6, status: todo, priority: 2, recur: 2w:mon,wed,fri -->",
		},
		{
			name: "note without status, priority, or tags",
			task: &models.Task{
				ID:        "note-1",
				Type:      models.TaskTypeNote,
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
			},
			want: "<!-- id: note-1, created: 2025-01-15 10:30 -->",
		},
		{
			name: "task with updated same as created",
			task: &models.Task{
				ID:        "task-7",
				Type:      models.TaskTypeTask,
				Status:    models.StatusTodo,
				Priority:  1,
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
			},
			want: "<!-- id: task-7, status: todo, priority: 1, created: 2025-01-15 10:30 -->",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WriteMetadata(tt.task)
			if got != tt.want {
				t.Errorf("WriteMetadata() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseWriteMetadata_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		task *models.Task
	}{
		{
			name: "complete task",
			task: &models.Task{
				ID:        "task-round-1",
				Type:      models.TaskTypeTask,
				Status:    models.StatusInProgress,
				Priority:  2,
				Tags:      []string{"work", "urgent"},
				Section:   "Backend",
				Pinned:    true,
				Active:    true,
				DependsOn: []string{"dep1", "dep2"},
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt: time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
				DueDate:   timePtr(time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)),
				Recurrence: &models.RecurrencePattern{
					Interval: 1,
					Unit:     models.RecurrenceWeekly,
					Weekdays: []int{1, 3, 5},
				},
			},
		},
		{
			name: "minimal task",
			task: &models.Task{
				ID:       "task-round-2",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaStr := WriteMetadata(tt.task)

			if len(metaStr) < 8 || metaStr[:4] != "<!--" || metaStr[len(metaStr)-3:] != "-->" {
				t.Fatalf("Invalid metadata format: %q", metaStr)
			}
			content := metaStr[4 : len(metaStr)-3]
			content = content[1 : len(content)-1]

			parsed := &models.Task{}
			ParseMetadata(parsed, content)

			assertTaskFields(t, parsed, tt.task)
		})
	}
}

func TestParseTimeField(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantOk  bool
		wantStr string
	}{
		{
			name:    "datetime format",
			value:   "2025-01-15 10:30",
			wantOk:  true,
			wantStr: "2025-01-15 10:30",
		},
		{
			name:    "date only format",
			value:   "2025-01-15",
			wantOk:  true,
			wantStr: "2025-01-15 00:00",
		},
		{
			name:   "invalid format",
			value:  "not-a-date",
			wantOk: false,
		},
		{
			name:   "empty string",
			value:  "",
			wantOk: false,
		},
		{
			name:   "partial date",
			value:  "2025-01",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseTimeField(tt.value)
			if ok != tt.wantOk {
				t.Errorf("ParseTimeField(%q) ok = %v, want %v", tt.value, ok, tt.wantOk)
			}
			if tt.wantOk {
				gotStr := got.Format(util.DateTimeFormat)
				if gotStr != tt.wantStr {
					t.Errorf("ParseTimeField(%q) = %q, want %q", tt.value, gotStr, tt.wantStr)
				}
			}
		})
	}
}

func TestFormatRecurrence(t *testing.T) {
	tests := []struct {
		name string
		r    *models.RecurrencePattern
		want string
	}{
		{
			name: "nil recurrence",
			r:    nil,
			want: "",
		},
		{
			name: "daily",
			r: &models.RecurrencePattern{
				Interval: 1,
				Unit:     models.RecurrenceDaily,
			},
			want: "1d",
		},
		{
			name: "every 3 days",
			r: &models.RecurrencePattern{
				Interval: 3,
				Unit:     models.RecurrenceDaily,
			},
			want: "3d",
		},
		{
			name: "weekly",
			r: &models.RecurrencePattern{
				Interval: 1,
				Unit:     models.RecurrenceWeekly,
			},
			want: "1w",
		},
		{
			name: "weekly with weekdays",
			r: &models.RecurrencePattern{
				Interval: 1,
				Unit:     models.RecurrenceWeekly,
				Weekdays: []int{1, 3, 5},
			},
			want: "1w:mon,wed,fri",
		},
		{
			name: "every 2 weeks on specific days",
			r: &models.RecurrencePattern{
				Interval: 2,
				Unit:     models.RecurrenceWeekly,
				Weekdays: []int{0, 6},
			},
			want: "2w:sun,sat",
		},
		{
			name: "monthly",
			r: &models.RecurrencePattern{
				Interval: 1,
				Unit:     models.RecurrenceMonthly,
			},
			want: "1m",
		},
		{
			name: "yearly",
			r: &models.RecurrencePattern{
				Interval: 1,
				Unit:     models.RecurrenceYearly,
			},
			want: "1y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRecurrence(tt.r)
			if got != tt.want {
				t.Errorf("FormatRecurrence() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Helper functions

func timePtr(t time.Time) *time.Time {
	return &t
}

func strPtr(s string) *string {
	return &s
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func intSlicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func ptrToStr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func ptrToTime(p *time.Time) string {
	if p == nil {
		return "<nil>"
	}
	return p.Format(util.DateTimeFormat)
}

func recurrenceEqual(a, b *models.RecurrencePattern) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Interval == b.Interval && a.Unit == b.Unit && intSlicesEqual(a.Weekdays, b.Weekdays)
}

func assertTaskFields(t *testing.T, task, want *models.Task) {
	t.Helper()
	if task.ID != want.ID {
		t.Errorf("ID = %q, want %q", task.ID, want.ID)
	}
	if task.Status != want.Status {
		t.Errorf("Status = %q, want %q", task.Status, want.Status)
	}
	if task.Priority != want.Priority {
		t.Errorf("Priority = %d, want %d", task.Priority, want.Priority)
	}
	if task.Section != want.Section {
		t.Errorf("Section = %q, want %q", task.Section, want.Section)
	}
	if task.Pinned != want.Pinned {
		t.Errorf("Pinned = %v, want %v", task.Pinned, want.Pinned)
	}
	if task.Active != want.Active {
		t.Errorf("Active = %v, want %v", task.Active, want.Active)
	}

	if !stringSlicesEqual(task.Tags, want.Tags) {
		t.Errorf("Tags = %v, want %v", task.Tags, want.Tags)
	}
	if !stringSlicesEqual(task.DependsOn, want.DependsOn) {
		t.Errorf("DependsOn = %v, want %v", task.DependsOn, want.DependsOn)
	}

	if !stringPtrEqual(task.ParentID, want.ParentID) {
		t.Errorf("ParentID = %v, want %v", ptrToStr(task.ParentID), ptrToStr(want.ParentID))
	}
	if !timePtrEqual(task.DueDate, want.DueDate) {
		t.Errorf("DueDate = %v, want %v", ptrToTime(task.DueDate), ptrToTime(want.DueDate))
	}
	if !timePtrEqual(task.CompletedAt, want.CompletedAt) {
		t.Errorf("CompletedAt = %v, want %v", ptrToTime(task.CompletedAt), ptrToTime(want.CompletedAt))
	}

	if !task.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", task.CreatedAt, want.CreatedAt)
	}
	if !task.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", task.UpdatedAt, want.UpdatedAt)
	}

	if !recurrenceEqual(task.Recurrence, want.Recurrence) {
		t.Errorf("Recurrence = %+v, want %+v", task.Recurrence, want.Recurrence)
	}
}
