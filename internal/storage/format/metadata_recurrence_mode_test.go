package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

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
