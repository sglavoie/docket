package format

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseMetadataSimplifiedRecurrence(t *testing.T) {
	tests := []struct {
		value    string
		interval int
		unit     models.RecurrenceUnit
	}{
		{"1d", 1, models.RecurrenceDaily},
		{"2w", 2, models.RecurrenceWeekly},
		{"3m", 3, models.RecurrenceMonthly},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			task := &models.Task{}
			warnings := ParseMetadata(task, "id: task, state: ready, priority: 1, recur: "+test.value)
			if len(warnings) != 0 {
				t.Fatalf("warnings = %v, want none", warnings)
			}
			if task.Recurrence == nil || task.Recurrence.Interval != test.interval || task.Recurrence.Unit != test.unit {
				t.Fatalf("Recurrence = %+v, want %d %s", task.Recurrence, test.interval, test.unit)
			}
		})
	}
}

func TestParseMetadataRejectsRichRecurrenceSyntax(t *testing.T) {
	for _, value := range []string{"1y", "1w:mon,fri", "1d~"} {
		t.Run(value, func(t *testing.T) {
			task := &models.Task{}
			warnings := ParseMetadata(task, "id: task, state: ready, priority: 1, recur: "+value)
			if task.Recurrence != nil {
				t.Fatalf("Recurrence = %+v, want nil", task.Recurrence)
			}
			if len(warnings) != 1 || warnings[0].Field != "recur" ||
				!strings.Contains(warnings[0].Message, "invalid recurrence") {
				t.Fatalf("warnings = %v, want invalid recurrence warning", warnings)
			}
		})
	}
}

func TestLegacyRecurrenceSubfieldsAreRecognizedAndDiscarded(t *testing.T) {
	task := &models.Task{}
	warnings := ParseMetadata(task, "id: task, state: ready, priority: 1, recur: 1w, recur.skipped: 3, recur.streak: 4, recur.max_streak: 5, recur.original_day: 31")
	if task.Recurrence == nil {
		t.Fatal("Recurrence is nil")
	}
	if task.Recurrence.Interval != 1 || task.Recurrence.Unit != models.RecurrenceWeekly {
		t.Fatalf("canonical recurrence changed: %+v", task.Recurrence)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Message, "not supported") {
		t.Fatalf("warnings = %v, want one unsupported legacy-field warning", warnings)
	}
}

func TestWriteMetadataEmitsOnlyCanonicalRecurrence(t *testing.T) {
	task := &models.Task{
		ID:            "task",
		WorkflowState: models.StateReady,
		Priority:      1,
		Recurrence: &models.RecurrencePattern{
			Interval: 2, Unit: models.RecurrenceWeekly,
		},
	}
	got := WriteMetadataV2(task)
	if !strings.Contains(got, "recur: 2w") {
		t.Fatalf("metadata = %q, want recur: 2w", got)
	}
	for _, retired := range []string{"recur.skipped", "recur.streak", "recur.max_streak", "recur.original_day"} {
		if strings.Contains(got, retired) {
			t.Errorf("metadata contains retired field %q: %s", retired, got)
		}
	}
}

func TestSimplifiedRecurrenceRoundTrip(t *testing.T) {
	for _, recurrence := range []*models.RecurrencePattern{
		{Interval: 1, Unit: models.RecurrenceDaily},
		{Interval: 2, Unit: models.RecurrenceWeekly},
		{Interval: 3, Unit: models.RecurrenceMonthly},
	} {
		task := &models.Task{
			ID: "task", WorkflowState: models.StateReady, Priority: 1,
			Recurrence: recurrence,
		}
		parsed := &models.Task{}
		warnings := parseWrittenMetadata(task, parsed)
		if len(warnings) != 0 {
			t.Fatalf("round-trip warnings = %v", warnings)
		}
		if !recurrenceEqual(parsed.Recurrence, recurrence) {
			t.Errorf("round-trip recurrence = %+v, want %+v", parsed.Recurrence, recurrence)
		}
	}
}

func TestFormatRecurrenceCanonicalUnits(t *testing.T) {
	tests := []struct {
		pattern *models.RecurrencePattern
		want    string
	}{
		{nil, ""},
		{&models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}, "1d"},
		{&models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly}, "2w"},
		{&models.RecurrencePattern{Interval: 3, Unit: models.RecurrenceMonthly}, "3m"},
		{&models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceUnit("year")}, ""},
	}
	for _, test := range tests {
		if got := FormatRecurrence(test.pattern); got != test.want {
			t.Errorf("FormatRecurrence(%+v) = %q, want %q", test.pattern, got, test.want)
		}
	}
}
