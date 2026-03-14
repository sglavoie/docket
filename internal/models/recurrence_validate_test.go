package models

import "testing"

func TestRecurrencePatternValidate(t *testing.T) {
	tests := []struct {
		name    string
		pattern *RecurrencePattern
		wantErr bool
	}{
		{"nil pattern", nil, true},
		{"valid daily", &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}, false},
		{"valid weekly", &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}, false},
		{"valid monthly", &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}, false},
		{"valid yearly", &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}, false},
		{"valid weekly with weekdays", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{0, 3, 6}}, false},
		{"zero interval", &RecurrencePattern{Interval: 0, Unit: RecurrenceDaily}, true},
		{"negative interval", &RecurrencePattern{Interval: -1, Unit: RecurrenceWeekly}, true},
		{"invalid unit", &RecurrencePattern{Interval: 1, Unit: "decade"}, true},
		{"empty unit", &RecurrencePattern{Interval: 1, Unit: ""}, true},
		{"weekday too low", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{-1}}, true},
		{"weekday too high", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{7}}, true},
		{"mixed valid and invalid weekday", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1, 8}}, true},
		{"empty weekdays ok", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pattern.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetRecurrenceValidation(t *testing.T) {
	task, err := NewTask("t1", 1, TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Valid pattern should succeed
	if err := task.SetRecurrence(&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}); err != nil {
		t.Errorf("SetRecurrence with valid pattern should not error: %v", err)
	}
	if task.Recurrence == nil {
		t.Error("Recurrence should be set after valid SetRecurrence")
	}

	// Invalid pattern should fail and not change existing recurrence
	old := task.Recurrence
	if err := task.SetRecurrence(&RecurrencePattern{Interval: 0, Unit: RecurrenceDaily}); err == nil {
		t.Error("SetRecurrence with invalid pattern should return error")
	}
	if task.Recurrence != old {
		t.Error("Recurrence should not change after failed SetRecurrence")
	}

	// nil pattern should succeed (clears recurrence)
	if err := task.SetRecurrence(nil); err != nil {
		t.Errorf("SetRecurrence(nil) should not error: %v", err)
	}
	if task.Recurrence != nil {
		t.Error("Recurrence should be nil after SetRecurrence(nil)")
	}
}
