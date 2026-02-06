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
			name:    "complete task metadata with custom section migration",
			metaStr: "id: task-456, status: done, priority: 1, tags: work;urgent, section: Backend",
			want: &models.Task{
				ID:            "task-456",
				Status:        models.StatusDone,
				Priority:      1,
				Tags:          []string{"work", "urgent"},
				CustomSection: "Backend",
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
			name: "task with GTD section",
			task: &models.Task{
				ID:       "task-789",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				Section:  "Next",
			},
			want: "<!-- id: task-789, status: todo, priority: 2, section: Next -->",
		},
		{
			name: "task with custom section",
			task: &models.Task{
				ID:            "task-790",
				Type:          models.TaskTypeTask,
				Status:        models.StatusTodo,
				Priority:      2,
				CustomSection: "Backend",
			},
			want: "<!-- id: task-790, status: todo, priority: 2, custom_section: Backend -->",
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
				ID:            "task-round-1",
				Type:          models.TaskTypeTask,
				Status:        models.StatusInProgress,
				Priority:      2,
				Tags:          []string{"work", "urgent"},
				CustomSection: "Backend",
				Pinned:        true,
				Active:        true,
				DependsOn:     []string{"dep1", "dep2"},
				CreatedAt:     time.Date(2025, 1, 15, 10, 30, 0, 0, time.Local),
				UpdatedAt:     time.Date(2025, 1, 16, 14, 20, 0, 0, time.Local),
				DueDate:       timePtr(time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)),
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

func TestCustomSectionRoundTrip(t *testing.T) {
	task := &models.Task{
		ID:            "cs-1",
		Type:          models.TaskTypeTask,
		Status:        models.StatusTodo,
		Priority:      2,
		Section:       "Next",
		CustomSection: "Backend",
	}

	metaStr := WriteMetadata(task)
	parsed := &models.Task{}
	content := metaStr[5 : len(metaStr)-4]
	ParseMetadata(parsed, content)

	if parsed.Section != "Next" {
		t.Errorf("Section = %q, want %q", parsed.Section, "Next")
	}
	if parsed.CustomSection != "Backend" {
		t.Errorf("CustomSection = %q, want %q", parsed.CustomSection, "Backend")
	}
}

func TestCustomSectionMigrationFromOldFormat(t *testing.T) {
	// Old format: non-GTD section stored in "section" field
	task := &models.Task{}
	ParseMetadata(task, "id: m1, status: todo, priority: 2, section: Backend")

	if task.Section != "" {
		t.Errorf("Section = %q, want empty (migrated away)", task.Section)
	}
	if task.CustomSection != "Backend" {
		t.Errorf("CustomSection = %q, want %q", task.CustomSection, "Backend")
	}
}

func TestGTDSectionNotMigrated(t *testing.T) {
	task := &models.Task{}
	ParseMetadata(task, "id: m2, status: todo, priority: 2, section: Next")

	if task.Section != "Next" {
		t.Errorf("Section = %q, want %q", task.Section, "Next")
	}
	if task.CustomSection != "" {
		t.Errorf("CustomSection = %q, want empty", task.CustomSection)
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
	if task.CustomSection != want.CustomSection {
		t.Errorf("CustomSection = %q, want %q", task.CustomSection, want.CustomSection)
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

func TestParseMetadata_BugAttrs(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    *models.BugAttrs
	}{
		{
			name:    "all bug attributes",
			metaStr: "id: b1, status: todo, priority: 2, bug.reproducible: yes, bug.severity: major, bug.workaround: Use Chrome",
			want: &models.BugAttrs{
				Reproducible: models.BugReproducibleYes,
				Severity:     models.BugSeverityMajor,
				Workaround:   "Use Chrome",
			},
		},
		{
			name:    "partial bug attributes",
			metaStr: "id: b2, status: todo, priority: 2, bug.severity: critical",
			want: &models.BugAttrs{
				Severity: models.BugSeverityCritical,
			},
		},
		{
			name:    "bug workaround with escaped newlines",
			metaStr: `id: b3, status: todo, priority: 2, bug.workaround: Line 1\nLine 2\nLine 3`,
			want: &models.BugAttrs{
				Workaround: "Line 1\nLine 2\nLine 3",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			ParseMetadata(task, tc.metaStr)

			if task.BugAttrs == nil {
				t.Fatal("BugAttrs is nil")
			}
			if task.BugAttrs.Reproducible != tc.want.Reproducible {
				t.Errorf("Reproducible = %q, want %q", task.BugAttrs.Reproducible, tc.want.Reproducible)
			}
			if task.BugAttrs.Severity != tc.want.Severity {
				t.Errorf("Severity = %q, want %q", task.BugAttrs.Severity, tc.want.Severity)
			}
			if task.BugAttrs.Workaround != tc.want.Workaround {
				t.Errorf("Workaround = %q, want %q", task.BugAttrs.Workaround, tc.want.Workaround)
			}
		})
	}
}

func TestParseMetadata_IdeaAttrs(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    *models.IdeaAttrs
	}{
		{
			name:    "all idea attributes",
			metaStr: "id: i1, status: todo, priority: 2, idea.category: feature, idea.effort: medium, idea.value: high, idea.validated: true",
			want: &models.IdeaAttrs{
				Category:  models.IdeaCategoryFeature,
				Effort:    models.EffortMedium,
				Value:     models.IdeaValueHigh,
				Validated: true,
			},
		},
		{
			name:    "partial idea attributes",
			metaStr: "id: i2, status: todo, priority: 2, idea.category: refactor",
			want: &models.IdeaAttrs{
				Category: models.IdeaCategoryRefactor,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			ParseMetadata(task, tc.metaStr)

			if task.IdeaAttrs == nil {
				t.Fatal("IdeaAttrs is nil")
			}
			if task.IdeaAttrs.Category != tc.want.Category {
				t.Errorf("Category = %q, want %q", task.IdeaAttrs.Category, tc.want.Category)
			}
			if task.IdeaAttrs.Effort != tc.want.Effort {
				t.Errorf("Effort = %q, want %q", task.IdeaAttrs.Effort, tc.want.Effort)
			}
			if task.IdeaAttrs.Value != tc.want.Value {
				t.Errorf("Value = %q, want %q", task.IdeaAttrs.Value, tc.want.Value)
			}
			if task.IdeaAttrs.Validated != tc.want.Validated {
				t.Errorf("Validated = %v, want %v", task.IdeaAttrs.Validated, tc.want.Validated)
			}
		})
	}
}

func TestParseMetadata_TaskAttrs(t *testing.T) {
	task := &models.Task{}
	ParseMetadata(task, "id: t1, status: todo, priority: 2, task.effort: small, task.worktype: development")

	if task.TaskAttrs == nil {
		t.Fatal("TaskAttrs is nil")
	}
	if task.TaskAttrs.Effort != models.EffortSmall {
		t.Errorf("Effort = %q, want %q", task.TaskAttrs.Effort, models.EffortSmall)
	}
	if task.TaskAttrs.WorkType != models.WorkTypeDevelopment {
		t.Errorf("WorkType = %q, want %q", task.TaskAttrs.WorkType, models.WorkTypeDevelopment)
	}
}

func TestParseMetadata_StoryAttrs(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    *models.StoryAttrs
	}{
		{
			name:    "all story attributes",
			metaStr: "id: s1, status: todo, priority: 2, story.points: 5, story.persona: Admin user",
			want: &models.StoryAttrs{
				StoryPoints: 5,
				Persona:     "Admin user",
			},
		},
		{
			name:    "story persona with escaped newlines",
			metaStr: `id: s2, status: todo, priority: 2, story.persona: As an admin\nI want to manage users\nSo that I can control access`,
			want: &models.StoryAttrs{
				Persona: "As an admin\nI want to manage users\nSo that I can control access",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			ParseMetadata(task, tc.metaStr)

			if task.StoryAttrs == nil {
				t.Fatal("StoryAttrs is nil")
			}
			if task.StoryAttrs.StoryPoints != tc.want.StoryPoints {
				t.Errorf("StoryPoints = %d, want %d", task.StoryAttrs.StoryPoints, tc.want.StoryPoints)
			}
			if task.StoryAttrs.Persona != tc.want.Persona {
				t.Errorf("Persona = %q, want %q", task.StoryAttrs.Persona, tc.want.Persona)
			}
		})
	}
}

func TestParseMetadata_EpicAttrs(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    *models.EpicAttrs
	}{
		{
			name:    "all epic attributes",
			metaStr: "id: e1, status: todo, priority: 2, epic.scope: defined, epic.milestone: Q1 2025, epic.metrics: 100 active users",
			want: &models.EpicAttrs{
				Scope:          models.EpicScopeDefined,
				Milestone:      "Q1 2025",
				SuccessMetrics: "100 active users",
			},
		},
		{
			name:    "epic metrics with escaped newlines",
			metaStr: `id: e2, status: todo, priority: 2, epic.metrics: 100 users\n50% retention\n5 star rating`,
			want: &models.EpicAttrs{
				SuccessMetrics: "100 users\n50% retention\n5 star rating",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			ParseMetadata(task, tc.metaStr)

			if task.EpicAttrs == nil {
				t.Fatal("EpicAttrs is nil")
			}
			if task.EpicAttrs.Scope != tc.want.Scope {
				t.Errorf("Scope = %q, want %q", task.EpicAttrs.Scope, tc.want.Scope)
			}
			if task.EpicAttrs.Milestone != tc.want.Milestone {
				t.Errorf("Milestone = %q, want %q", task.EpicAttrs.Milestone, tc.want.Milestone)
			}
			if task.EpicAttrs.SuccessMetrics != tc.want.SuccessMetrics {
				t.Errorf("SuccessMetrics = %q, want %q", task.EpicAttrs.SuccessMetrics, tc.want.SuccessMetrics)
			}
		})
	}
}

func TestWriteMetadata_TypeAttrs(t *testing.T) {
	tests := []struct {
		name     string
		task     *models.Task
		contains []string
	}{
		{
			name: "bug attributes",
			task: &models.Task{
				ID:       "b1",
				Type:     models.TaskTypeBug,
				Status:   models.StatusTodo,
				Priority: 2,
				BugAttrs: &models.BugAttrs{
					Reproducible: models.BugReproducibleYes,
					Severity:     models.BugSeverityMajor,
					Workaround:   "Use Chrome",
				},
			},
			contains: []string{
				"bug.reproducible: yes",
				"bug.severity: major",
				"bug.workaround: Use Chrome",
			},
		},
		{
			name: "idea attributes",
			task: &models.Task{
				ID:       "i1",
				Type:     models.TaskTypeIdea,
				Status:   models.StatusTodo,
				Priority: 2,
				IdeaAttrs: &models.IdeaAttrs{
					Category:  models.IdeaCategoryFeature,
					Effort:    models.EffortMedium,
					Value:     models.IdeaValueHigh,
					Validated: true,
				},
			},
			contains: []string{
				"idea.category: feature",
				"idea.effort: medium",
				"idea.value: high",
				"idea.validated: true",
			},
		},
		{
			name: "task attributes",
			task: &models.Task{
				ID:       "t1",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				TaskAttrs: &models.TaskAttrs{
					Effort:   models.EffortSmall,
					WorkType: models.WorkTypeDevelopment,
				},
			},
			contains: []string{
				"task.effort: small",
				"task.worktype: development",
			},
		},
		{
			name: "story attributes",
			task: &models.Task{
				ID:       "s1",
				Type:     models.TaskTypeStory,
				Status:   models.StatusTodo,
				Priority: 2,
				StoryAttrs: &models.StoryAttrs{
					StoryPoints: 5,
					Persona:     "Admin user",
				},
			},
			contains: []string{
				"story.points: 5",
				"story.persona: Admin user",
			},
		},
		{
			name: "epic attributes",
			task: &models.Task{
				ID:       "e1",
				Type:     models.TaskTypeEpic,
				Status:   models.StatusTodo,
				Priority: 2,
				EpicAttrs: &models.EpicAttrs{
					Scope:          models.EpicScopeDefined,
					Milestone:      "Q1 2025",
					SuccessMetrics: "100 active users",
				},
			},
			contains: []string{
				"epic.scope: defined",
				"epic.milestone: Q1 2025",
				"epic.metrics: 100 active users",
			},
		},
		{
			name: "bug workaround with newlines escaped",
			task: &models.Task{
				ID:       "b2",
				Type:     models.TaskTypeBug,
				Status:   models.StatusTodo,
				Priority: 2,
				BugAttrs: &models.BugAttrs{
					Workaround: "Line 1\nLine 2\nLine 3",
				},
			},
			contains: []string{
				`bug.workaround: Line 1\nLine 2\nLine 3`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WriteMetadata(tc.task)
			for _, want := range tc.contains {
				if !containsString(got, want) {
					t.Errorf("WriteMetadata() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

func TestTypeAttrs_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		task *models.Task
	}{
		{
			name: "bug with all attrs",
			task: &models.Task{
				ID:       "bug-round",
				Type:     models.TaskTypeBug,
				Status:   models.StatusTodo,
				Priority: 2,
				BugAttrs: &models.BugAttrs{
					Reproducible: models.BugReproducibleSometimes,
					Severity:     models.BugSeverityMinor,
					Workaround:   "Workaround with\nmultiple lines",
				},
			},
		},
		{
			name: "idea with all attrs",
			task: &models.Task{
				ID:       "idea-round",
				Type:     models.TaskTypeIdea,
				Status:   models.StatusTodo,
				Priority: 2,
				IdeaAttrs: &models.IdeaAttrs{
					Category:  models.IdeaCategoryResearch,
					Effort:    models.EffortLarge,
					Value:     models.IdeaValueMedium,
					Validated: true,
				},
			},
		},
		{
			name: "task with all attrs",
			task: &models.Task{
				ID:       "task-round",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				TaskAttrs: &models.TaskAttrs{
					Effort:   models.EffortMedium,
					WorkType: models.WorkTypeTesting,
				},
			},
		},
		{
			name: "story with all attrs",
			task: &models.Task{
				ID:       "story-round",
				Type:     models.TaskTypeStory,
				Status:   models.StatusTodo,
				Priority: 2,
				StoryAttrs: &models.StoryAttrs{
					StoryPoints: 8,
					Persona:     "As a user\nI want to\nSo that",
				},
			},
		},
		{
			name: "epic with all attrs",
			task: &models.Task{
				ID:       "epic-round",
				Type:     models.TaskTypeEpic,
				Status:   models.StatusTodo,
				Priority: 2,
				EpicAttrs: &models.EpicAttrs{
					Scope:          models.EpicScopeEvolving,
					Milestone:      "Q2 2025",
					SuccessMetrics: "Metric 1\nMetric 2",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metaStr := WriteMetadata(tc.task)

			// Extract content from <!-- ... -->
			if len(metaStr) < 8 || metaStr[:4] != "<!--" || metaStr[len(metaStr)-3:] != "-->" {
				t.Fatalf("Invalid metadata format: %q", metaStr)
			}
			content := metaStr[5 : len(metaStr)-4]

			parsed := &models.Task{}
			ParseMetadata(parsed, content)

			// Check type-specific attributes
			switch tc.task.Type {
			case models.TaskTypeBug:
				if tc.task.BugAttrs != nil {
					if parsed.BugAttrs == nil {
						t.Fatal("BugAttrs is nil after roundtrip")
					}
					if parsed.BugAttrs.Reproducible != tc.task.BugAttrs.Reproducible {
						t.Errorf("Reproducible = %q, want %q", parsed.BugAttrs.Reproducible, tc.task.BugAttrs.Reproducible)
					}
					if parsed.BugAttrs.Severity != tc.task.BugAttrs.Severity {
						t.Errorf("Severity = %q, want %q", parsed.BugAttrs.Severity, tc.task.BugAttrs.Severity)
					}
					if parsed.BugAttrs.Workaround != tc.task.BugAttrs.Workaround {
						t.Errorf("Workaround = %q, want %q", parsed.BugAttrs.Workaround, tc.task.BugAttrs.Workaround)
					}
				}
			case models.TaskTypeIdea:
				if tc.task.IdeaAttrs != nil {
					if parsed.IdeaAttrs == nil {
						t.Fatal("IdeaAttrs is nil after roundtrip")
					}
					if parsed.IdeaAttrs.Category != tc.task.IdeaAttrs.Category {
						t.Errorf("Category = %q, want %q", parsed.IdeaAttrs.Category, tc.task.IdeaAttrs.Category)
					}
					if parsed.IdeaAttrs.Effort != tc.task.IdeaAttrs.Effort {
						t.Errorf("Effort = %q, want %q", parsed.IdeaAttrs.Effort, tc.task.IdeaAttrs.Effort)
					}
					if parsed.IdeaAttrs.Value != tc.task.IdeaAttrs.Value {
						t.Errorf("Value = %q, want %q", parsed.IdeaAttrs.Value, tc.task.IdeaAttrs.Value)
					}
					if parsed.IdeaAttrs.Validated != tc.task.IdeaAttrs.Validated {
						t.Errorf("Validated = %v, want %v", parsed.IdeaAttrs.Validated, tc.task.IdeaAttrs.Validated)
					}
				}
			case models.TaskTypeTask:
				if tc.task.TaskAttrs != nil {
					if parsed.TaskAttrs == nil {
						t.Fatal("TaskAttrs is nil after roundtrip")
					}
					if parsed.TaskAttrs.Effort != tc.task.TaskAttrs.Effort {
						t.Errorf("Effort = %q, want %q", parsed.TaskAttrs.Effort, tc.task.TaskAttrs.Effort)
					}
					if parsed.TaskAttrs.WorkType != tc.task.TaskAttrs.WorkType {
						t.Errorf("WorkType = %q, want %q", parsed.TaskAttrs.WorkType, tc.task.TaskAttrs.WorkType)
					}
				}
			case models.TaskTypeStory:
				if tc.task.StoryAttrs != nil {
					if parsed.StoryAttrs == nil {
						t.Fatal("StoryAttrs is nil after roundtrip")
					}
					if parsed.StoryAttrs.StoryPoints != tc.task.StoryAttrs.StoryPoints {
						t.Errorf("StoryPoints = %d, want %d", parsed.StoryAttrs.StoryPoints, tc.task.StoryAttrs.StoryPoints)
					}
					if parsed.StoryAttrs.Persona != tc.task.StoryAttrs.Persona {
						t.Errorf("Persona = %q, want %q", parsed.StoryAttrs.Persona, tc.task.StoryAttrs.Persona)
					}
				}
			case models.TaskTypeEpic:
				if tc.task.EpicAttrs != nil {
					if parsed.EpicAttrs == nil {
						t.Fatal("EpicAttrs is nil after roundtrip")
					}
					if parsed.EpicAttrs.Scope != tc.task.EpicAttrs.Scope {
						t.Errorf("Scope = %q, want %q", parsed.EpicAttrs.Scope, tc.task.EpicAttrs.Scope)
					}
					if parsed.EpicAttrs.Milestone != tc.task.EpicAttrs.Milestone {
						t.Errorf("Milestone = %q, want %q", parsed.EpicAttrs.Milestone, tc.task.EpicAttrs.Milestone)
					}
					if parsed.EpicAttrs.SuccessMetrics != tc.task.EpicAttrs.SuccessMetrics {
						t.Errorf("SuccessMetrics = %q, want %q", parsed.EpicAttrs.SuccessMetrics, tc.task.EpicAttrs.SuccessMetrics)
					}
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsString(s[1:], substr)))
}

func TestParseMetadata_ScoreDimensions(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    map[string]int
	}{
		{
			name:    "single score dimension",
			metaStr: "id: s1, status: todo, priority: 2, score.impact: 4",
			want:    map[string]int{"impact": 4},
		},
		{
			name:    "multiple score dimensions",
			metaStr: "id: s2, status: todo, priority: 2, score.impact: 4, score.confidence: 3, score.reach: 8",
			want:    map[string]int{"impact": 4, "confidence": 3, "reach": 8},
		},
		{
			name:    "score dimensions mixed with other metadata",
			metaStr: "id: s3, status: in_progress, priority: 1, tags: urgent, score.effort: 2, section: Next, score.value: 5",
			want:    map[string]int{"effort": 2, "value": 5},
		},
		{
			name:    "score dimension with underscore in name",
			metaStr: "id: s4, status: todo, priority: 2, score.business_value: 7",
			want:    map[string]int{"business_value": 7},
		},
		{
			name:    "no score dimensions",
			metaStr: "id: s5, status: todo, priority: 2",
			want:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			ParseMetadata(task, tc.metaStr)

			if tc.want == nil {
				if task.ScoreDimensions != nil {
					t.Errorf("ScoreDimensions = %v, want nil", task.ScoreDimensions)
				}
				return
			}
			if task.ScoreDimensions == nil {
				t.Fatal("ScoreDimensions is nil")
			}
			for k, v := range tc.want {
				if got, ok := task.ScoreDimensions[k]; !ok {
					t.Errorf("missing dimension %q", k)
				} else if got != v {
					t.Errorf("dimension %q = %d, want %d", k, got, v)
				}
			}
			if len(task.ScoreDimensions) != len(tc.want) {
				t.Errorf("ScoreDimensions has %d entries, want %d", len(task.ScoreDimensions), len(tc.want))
			}
		})
	}
}

func TestWriteMetadata_ScoreDimensions(t *testing.T) {
	task := &models.Task{
		ID:              "score-1",
		Status:          models.StatusTodo,
		Priority:        2,
		ScoreDimensions: map[string]int{"confidence": 3, "impact": 4},
	}

	got := WriteMetadata(task)
	// Keys should appear in sorted order
	if !containsString(got, "score.confidence: 3") {
		t.Errorf("WriteMetadata() = %q, should contain %q", got, "score.confidence: 3")
	}
	if !containsString(got, "score.impact: 4") {
		t.Errorf("WriteMetadata() = %q, should contain %q", got, "score.impact: 4")
	}
}

func TestScoreDimensions_RoundTrip(t *testing.T) {
	task := &models.Task{
		ID:       "score-rt",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		ScoreDimensions: map[string]int{
			"impact":     4,
			"confidence": 3,
			"reach":      8,
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]

	parsed := &models.Task{}
	ParseMetadata(parsed, content)

	if parsed.ScoreDimensions == nil {
		t.Fatal("ScoreDimensions is nil after roundtrip")
	}
	for k, v := range task.ScoreDimensions {
		if got, ok := parsed.ScoreDimensions[k]; !ok {
			t.Errorf("missing dimension %q after roundtrip", k)
		} else if got != v {
			t.Errorf("dimension %q = %d after roundtrip, want %d", k, got, v)
		}
	}
}

func TestScoreDimensions_EmptyNotWritten(t *testing.T) {
	task := &models.Task{
		ID:       "score-empty",
		Status:   models.StatusTodo,
		Priority: 2,
	}

	got := WriteMetadata(task)
	if containsString(got, "score.") {
		t.Errorf("WriteMetadata() = %q, should not contain score dimensions for nil map", got)
	}

	task.ScoreDimensions = map[string]int{}
	got = WriteMetadata(task)
	if containsString(got, "score.") {
		t.Errorf("WriteMetadata() = %q, should not contain score dimensions for empty map", got)
	}
}

func TestEscapeUnescapeNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no newlines", "simple text"},
		{"single newline", "line1\nline2"},
		{"multiple newlines", "line1\nline2\nline3"},
		{"newline at start", "\ntext"},
		{"newline at end", "text\n"},
		{"consecutive newlines", "text\n\nmore"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			escaped := escapeNewlines(tc.input)
			unescaped := unescapeNewlines(escaped)
			if unescaped != tc.input {
				t.Errorf("roundtrip failed: input=%q, escaped=%q, unescaped=%q", tc.input, escaped, unescaped)
			}
		})
	}
}
