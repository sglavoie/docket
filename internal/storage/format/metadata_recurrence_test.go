package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseMetadata_RecurrenceStreakFields(t *testing.T) {
	task := &models.Task{}
	ParseMetadata(task, "id: abc, status: todo, priority: 1, recur: 1w, recur.skipped: 3, recur.streak: 5, recur.max_streak: 8")

	if task.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if task.Recurrence.Skipped != 3 {
		t.Errorf("Skipped = %d, want 3", task.Recurrence.Skipped)
	}
	if task.Recurrence.Streak != 5 {
		t.Errorf("Streak = %d, want 5", task.Recurrence.Streak)
	}
	if task.Recurrence.MaxStreak != 8 {
		t.Errorf("MaxStreak = %d, want 8", task.Recurrence.MaxStreak)
	}
}

func TestParseMetadata_InvalidRecurEmitsWarning(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantSub string
	}{
		{"garbage", "notarecur", "notarecur"},
		{"bad unit", "1z", "1z"},
		{"empty trailing tilde", "~", "~"},
		{"weekly-no-days", "weekly:", "weekly:"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			warnings := ParseMetadata(task, "id: abc, status: todo, priority: 1, recur: "+tc.value)
			if task.Recurrence != nil {
				t.Errorf("Recurrence should be nil for invalid value %q, got %+v", tc.value, task.Recurrence)
			}
			found := false
			for _, w := range warnings {
				if w.Field == "recur" && strings.Contains(w.Message, "invalid recurrence") && strings.Contains(w.Message, tc.wantSub) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected invalid-recurrence warning containing %q, got warnings: %v", tc.wantSub, warnings)
			}
		})
	}
}

func TestParseMetadata_RecurrenceStreakFieldsIgnoredWithoutRecur(t *testing.T) {
	task := &models.Task{}
	warnings := ParseMetadata(task, "id: abc, status: todo, priority: 1, recur.streak: 5")

	// Without a recur field, Recurrence is nil so the streak fields are ignored
	if task.Recurrence != nil {
		t.Errorf("Recurrence should be nil when recur key is absent")
	}
	// Should produce an orphaned sub-fields warning
	found := false
	for _, w := range warnings {
		if w.Field == "recur" && strings.Contains(w.Message, "sub-fields") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about orphaned recurrence sub-fields")
	}
}

func TestWriteMetadata_RecurrenceStreakFields(t *testing.T) {
	task := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval:  1,
			Unit:      models.RecurrenceWeekly,
			Skipped:   2,
			Streak:    4,
			MaxStreak: 7,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadata(task)
	if !strings.Contains(result, "recur.skipped: 2") {
		t.Errorf("expected recur.skipped: 2, got: %s", result)
	}
	if !strings.Contains(result, "recur.streak: 4") {
		t.Errorf("expected recur.streak: 4, got: %s", result)
	}
	if !strings.Contains(result, "recur.max_streak: 7") {
		t.Errorf("expected recur.max_streak: 7, got: %s", result)
	}
}

func TestWriteMetadata_RecurrenceStreakFieldsOmittedWhenZero(t *testing.T) {
	task := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval: 1,
			Unit:     models.RecurrenceDaily,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadata(task)
	if strings.Contains(result, "recur.skipped") {
		t.Errorf("should not contain recur.skipped when 0, got: %s", result)
	}
	if strings.Contains(result, "recur.streak") {
		t.Errorf("should not contain recur.streak when 0, got: %s", result)
	}
	if strings.Contains(result, "recur.max_streak") {
		t.Errorf("should not contain recur.max_streak when 0, got: %s", result)
	}
}

func TestParseMetadata_RecurrenceSubFieldsBeforeRecurKey(t *testing.T) {
	task := &models.Task{}
	// Sub-fields appear BEFORE the recur key — should still be applied
	ParseMetadata(task, "id: abc, status: todo, priority: 1, recur.skipped: 2, recur.streak: 4, recur.max_streak: 9, recur: 1w")

	if task.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if task.Recurrence.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", task.Recurrence.Skipped)
	}
	if task.Recurrence.Streak != 4 {
		t.Errorf("Streak = %d, want 4", task.Recurrence.Streak)
	}
	if task.Recurrence.MaxStreak != 9 {
		t.Errorf("MaxStreak = %d, want 9", task.Recurrence.MaxStreak)
	}
}

func TestMetadata_RecurrenceStreakRoundtrip(t *testing.T) {
	original := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval:  2,
			Unit:      models.RecurrenceWeekly,
			Weekdays:  []int{1, 3, 5},
			Skipped:   3,
			Streak:    6,
			MaxStreak: 10,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	meta := WriteMetadata(original)
	metaStr := strings.TrimPrefix(meta, "<!-- ")
	metaStr = strings.TrimSuffix(metaStr, " -->")

	parsed := &models.Task{}
	ParseMetadata(parsed, metaStr)

	if !recurrenceEqual(parsed.Recurrence, original.Recurrence) {
		t.Errorf("Roundtrip failed:\n  got:  %+v\n  want: %+v", parsed.Recurrence, original.Recurrence)
	}
}

func TestParseMetadata_RecurrenceOriginalDay(t *testing.T) {
	task := &models.Task{}
	ParseMetadata(task, "id: abc, status: todo, priority: 1, recur: 1m, recur.original_day: 31")

	if task.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if task.Recurrence.OriginalDay != 31 {
		t.Errorf("OriginalDay = %d, want 31", task.Recurrence.OriginalDay)
	}
}

func TestWriteMetadata_RecurrenceOriginalDay(t *testing.T) {
	task := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval:    1,
			Unit:        models.RecurrenceMonthly,
			OriginalDay: 31,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadata(task)
	if !strings.Contains(result, "recur.original_day: 31") {
		t.Errorf("expected recur.original_day: 31, got: %s", result)
	}
}

func TestWriteMetadata_RecurrenceOriginalDayOmittedWhenZero(t *testing.T) {
	task := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval: 1,
			Unit:     models.RecurrenceMonthly,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadata(task)
	if strings.Contains(result, "recur.original_day") {
		t.Errorf("should not contain recur.original_day when 0, got: %s", result)
	}
}

func TestMetadata_RecurrenceOriginalDayRoundtrip(t *testing.T) {
	original := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval:    1,
			Unit:        models.RecurrenceMonthly,
			OriginalDay: 31,
			Streak:      3,
			MaxStreak:   5,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	meta := WriteMetadata(original)
	metaStr := strings.TrimPrefix(meta, "<!-- ")
	metaStr = strings.TrimSuffix(metaStr, " -->")

	parsed := &models.Task{}
	ParseMetadata(parsed, metaStr)

	if !recurrenceEqual(parsed.Recurrence, original.Recurrence) {
		t.Errorf("Roundtrip failed:\n  got:  %+v\n  want: %+v", parsed.Recurrence, original.Recurrence)
	}
}

// TestParseMetadata_RecurrenceWeekdayCommas verifies that comma-separated weekday
// lists inside a recur value (e.g., "1w:mon,wed,fri") are not mis-split by the
// metadata parser, which uses ", key:" as its delimiter rather than bare commas.
func TestParseMetadata_RecurrenceWeekdayCommas(t *testing.T) {
	task := &models.Task{}
	warnings := ParseMetadata(task, "id: abc, status: todo, priority: 1, recur: 1w:mon,wed,fri")

	if task.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if task.Recurrence.Unit != models.RecurrenceWeekly {
		t.Errorf("Unit = %v, want weekly", task.Recurrence.Unit)
	}
	if task.Recurrence.Interval != 1 {
		t.Errorf("Interval = %d, want 1", task.Recurrence.Interval)
	}
	// mon=1, wed=3, fri=5
	if len(task.Recurrence.Weekdays) != 3 {
		t.Errorf("Weekdays = %v, want [1 3 5] (mon,wed,fri)", task.Recurrence.Weekdays)
	}
	for _, w := range warnings {
		if w.Field == "recur" {
			t.Errorf("unexpected recurrence warning: %s", w.Message)
		}
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
		// from-completion mode appends ~
		{
			name: "daily from-completion",
			r:    &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Mode: models.RecurModeFromCompletion},
			want: "1d~",
		},
		{
			name: "every 3 days from-completion",
			r:    &models.RecurrencePattern{Interval: 3, Unit: models.RecurrenceDaily, Mode: models.RecurModeFromCompletion},
			want: "3d~",
		},
		{
			name: "weekly with weekdays from-completion",
			r:    &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 3, 5}, Mode: models.RecurModeFromCompletion},
			want: "1w:mon,wed,fri~",
		},
		{
			name: "biweekly mon,fri from-completion",
			r:    &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 5}, Mode: models.RecurModeFromCompletion},
			want: "2w:mon,fri~",
		},
		{
			name: "monthly from-completion",
			r:    &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceMonthly, Mode: models.RecurModeFromCompletion},
			want: "1m~",
		},
		// explicit from-due does not append ~
		{
			name: "daily explicit from-due",
			r:    &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Mode: models.RecurModeFromDue},
			want: "1d",
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

func TestParseMetadata_RecurrenceFromCompletion(t *testing.T) {
	tests := []struct {
		name     string
		metaStr  string
		wantMode models.RecurMode
	}{
		{
			name:     "1d~ sets from-completion",
			metaStr:  "id: abc, status: todo, priority: 1, recur: 1d~",
			wantMode: models.RecurModeFromCompletion,
		},
		{
			name:     "1w:mon,fri~ sets from-completion",
			metaStr:  "id: abc, status: todo, priority: 1, recur: 1w:mon,fri~",
			wantMode: models.RecurModeFromCompletion,
		},
		{
			name:     "1m~ sets from-completion",
			metaStr:  "id: abc, status: todo, priority: 1, recur: 1m~",
			wantMode: models.RecurModeFromCompletion,
		},
		{
			name:     "1d (no ~) leaves mode empty",
			metaStr:  "id: abc, status: todo, priority: 1, recur: 1d",
			wantMode: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			warnings := ParseMetadata(task, tc.metaStr)
			for _, w := range warnings {
				if w.Field == "recur" {
					t.Errorf("unexpected recur warning: %s", w.Message)
				}
			}
			if task.Recurrence == nil {
				t.Fatal("Recurrence is nil")
			}
			if task.Recurrence.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q", task.Recurrence.Mode, tc.wantMode)
			}
		})
	}
}

func TestMetadata_RecurrenceFromCompletionRoundtrip(t *testing.T) {
	original := &models.Task{
		ID:       "fc-rt-1",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval:  2,
			Unit:      models.RecurrenceWeekly,
			Weekdays:  []int{1, 5},
			Streak:    3,
			MaxStreak: 7,
			Mode:      models.RecurModeFromCompletion,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	meta := WriteMetadata(original)
	metaStr := strings.TrimPrefix(meta, "<!-- ")
	metaStr = strings.TrimSuffix(metaStr, " -->")

	parsed := &models.Task{}
	ParseMetadata(parsed, metaStr)

	if !recurrenceEqual(parsed.Recurrence, original.Recurrence) {
		t.Errorf("Roundtrip failed:\n  got:  %+v\n  want: %+v", parsed.Recurrence, original.Recurrence)
	}
}
