package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
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
			name:    "task with stage",
			metaStr: "id: t11, status: todo, priority: 2, stage: Design",
			want: &models.Task{
				ID:       "t11",
				Status:   models.StatusTodo,
				Priority: 2,
				Stage:    "Design",
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

func TestCustomSectionMigrationFromOldFormat(t *testing.T) {
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

func TestNonGTDSectionClearedWhenCustomSectionPresent(t *testing.T) {
	// When both section and custom_section are set and section is non-GTD,
	// the section field should be cleared to prevent a non-GTD value lingering
	// in the GTD field. CustomSection takes precedence.
	task := &models.Task{}
	ParseMetadata(task, "id: m3, status: todo, priority: 2, section: Backend, custom_section: Frontend")

	if task.Section != "" {
		t.Errorf("Section = %q, want empty (non-GTD value should be cleared)", task.Section)
	}
	if task.CustomSection != "Frontend" {
		t.Errorf("CustomSection = %q, want %q (explicit custom_section takes precedence)", task.CustomSection, "Frontend")
	}
}

func TestGTDSectionPreservedWithCustomSection(t *testing.T) {
	// When section is a valid GTD section and custom_section is also set,
	// both should be preserved (though this is an unusual state).
	task := &models.Task{}
	ParseMetadata(task, "id: m4, status: todo, priority: 2, section: Next, custom_section: Frontend")

	if task.Section != "Next" {
		t.Errorf("Section = %q, want %q (GTD section should be preserved)", task.Section, "Next")
	}
	if task.CustomSection != "Frontend" {
		t.Errorf("CustomSection = %q, want %q", task.CustomSection, "Frontend")
	}
}

func TestParseMetadata_EmptyAndMalformedComment(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
	}{
		{name: "empty string", metaStr: ""},
		{name: "whitespace only", metaStr: "   "},
		{name: "no colon in pair", metaStr: "just-garbage"},
		{name: "malformed pair no value", metaStr: "id:"},
		{name: "multiple malformed pairs", metaStr: "foo, bar, baz"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMetadata panicked for input %q: %v", tc.metaStr, r)
				}
			}()
			task := &models.Task{}
			_ = ParseMetadata(task, tc.metaStr)
		})
	}
}

func TestParseMetadata_DedupsDependsOn(t *testing.T) {
	task := &models.Task{
		DependsOn: []string{},
		RelatedTo: []string{},
	}
	_ = ParseMetadata(task, "id: abc, depends: dep1;dep2;dep1;dep3;dep2")
	if len(task.DependsOn) != 3 {
		t.Errorf("expected 3 unique depends, got %d: %v", len(task.DependsOn), task.DependsOn)
	}
	want := []string{"dep1", "dep2", "dep3"}
	for i, w := range want {
		if i >= len(task.DependsOn) || task.DependsOn[i] != w {
			t.Errorf("DependsOn[%d] = %q, want %q", i, task.DependsOn[i], w)
		}
	}
}

func TestParseMetadata_DedupsRelatedTo(t *testing.T) {
	task := &models.Task{
		DependsOn: []string{},
		RelatedTo: []string{},
	}
	_ = ParseMetadata(task, "id: abc, related: rel1;rel2;rel1;rel3;rel2")
	if len(task.RelatedTo) != 3 {
		t.Errorf("expected 3 unique related, got %d: %v", len(task.RelatedTo), task.RelatedTo)
	}
	want := []string{"rel1", "rel2", "rel3"}
	for i, w := range want {
		if i >= len(task.RelatedTo) || task.RelatedTo[i] != w {
			t.Errorf("RelatedTo[%d] = %q, want %q", i, task.RelatedTo[i], w)
		}
	}
}

func TestParseMetadata_NoDuplicateWithSingleDep(t *testing.T) {
	task := &models.Task{
		DependsOn: []string{},
		RelatedTo: []string{},
	}
	_ = ParseMetadata(task, "id: abc, depends: only-one")
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dep, got %d: %v", len(task.DependsOn), task.DependsOn)
	}
	if task.DependsOn[0] != "only-one" {
		t.Errorf("DependsOn[0] = %q, want %q", task.DependsOn[0], "only-one")
	}
}

func TestParseMetadata_InvalidStatusWarnsAndPreservesExistingValue(t *testing.T) {
	task := &models.Task{
		ID:       "existing-id",
		Status:   models.StatusTodo,
		Priority: 2,
	}

	warnings := ParseMetadata(task, "id: parsed-id, status: invalid_status, priority: 2")

	if task.ID != "parsed-id" {
		t.Errorf("ID = %q, want %q", task.ID, "parsed-id")
	}
	if task.Status != models.StatusTodo {
		t.Errorf("Status = %q, want %q", task.Status, models.StatusTodo)
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
	if !hasWarningField(warnings, "status") {
		t.Errorf("expected warning for field %q, got %v", "status", warnings)
	}
}

func TestParseMetadata_InvalidTimeFieldsWarnAndPreserveExistingValues(t *testing.T) {
	created := time.Date(2025, 1, 10, 8, 0, 0, 0, time.Local)
	updated := time.Date(2025, 1, 11, 9, 15, 0, 0, time.Local)
	completed := time.Date(2025, 1, 12, 10, 30, 0, 0, time.Local)
	due := time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)
	start := time.Date(2025, 1, 9, 0, 0, 0, 0, time.Local)
	snoozed := time.Date(2025, 1, 20, 0, 0, 0, 0, time.Local)

	task := &models.Task{
		CreatedAt:    created,
		UpdatedAt:    updated,
		CompletedAt:  timePtr(completed),
		DueDate:      timePtr(due),
		StartDate:    timePtr(start),
		SnoozedUntil: timePtr(snoozed),
		HasTime:      true,
	}

	warnings := ParseMetadata(task, "created: not-a-date, updated: nope, completed: later, due: tomorrow-ish, start: 2025/01/09, snoozed: soon")

	if !task.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", task.CreatedAt, created)
	}
	if !task.UpdatedAt.Equal(updated) {
		t.Errorf("UpdatedAt = %v, want %v", task.UpdatedAt, updated)
	}
	if task.CompletedAt == nil || !task.CompletedAt.Equal(completed) {
		t.Errorf("CompletedAt = %v, want %v", task.CompletedAt, completed)
	}
	if task.DueDate == nil || !task.DueDate.Equal(due) {
		t.Errorf("DueDate = %v, want %v", task.DueDate, due)
	}
	if task.StartDate == nil || !task.StartDate.Equal(start) {
		t.Errorf("StartDate = %v, want %v", task.StartDate, start)
	}
	if task.SnoozedUntil == nil || !task.SnoozedUntil.Equal(snoozed) {
		t.Errorf("SnoozedUntil = %v, want %v", task.SnoozedUntil, snoozed)
	}
	if !task.HasTime {
		t.Error("HasTime should remain unchanged when due parsing fails")
	}

	wantFields := []string{"created", "updated", "completed", "due", "start", "snoozed"}
	if len(warnings) != len(wantFields) {
		t.Fatalf("got %d warnings, want %d: %v", len(warnings), len(wantFields), warnings)
	}
	for _, field := range wantFields {
		if !hasWarningField(warnings, field) {
			t.Errorf("expected warning for field %q, got %v", field, warnings)
		}
	}
}
