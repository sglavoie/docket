package models

import "testing"

// TestParseRecurrence_FromCompletionMode verifies that a trailing "~" sets
// Mode = RecurModeFromCompletion across all supported recurrence forms.
func TestParseRecurrence_FromCompletionMode(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantNil      bool
		wantInterval int
		wantUnit     RecurrenceUnit
		wantWeekdays []int
		wantMode     RecurMode
	}{
		// Keywords with ~
		{"daily~", "daily~", false, 1, RecurrenceDaily, nil, RecurModeFromCompletion},
		{"weekly~", "weekly~", false, 1, RecurrenceWeekly, nil, RecurModeFromCompletion},
		{"biweekly~", "biweekly~", false, 2, RecurrenceWeekly, nil, RecurModeFromCompletion},
		{"monthly~", "monthly~", false, 1, RecurrenceMonthly, nil, RecurModeFromCompletion},
		{"yearly~", "yearly~", false, 1, RecurrenceYearly, nil, RecurModeFromCompletion},

		// Interval forms with ~
		{"1d~", "1d~", false, 1, RecurrenceDaily, nil, RecurModeFromCompletion},
		{"2d~", "2d~", false, 2, RecurrenceDaily, nil, RecurModeFromCompletion},
		{"3w~", "3w~", false, 3, RecurrenceWeekly, nil, RecurModeFromCompletion},
		{"1m~", "1m~", false, 1, RecurrenceMonthly, nil, RecurModeFromCompletion},
		{"2y~", "2y~", false, 2, RecurrenceYearly, nil, RecurModeFromCompletion},

		// Weekday forms with ~
		{"weekly:mon~", "weekly:mon~", false, 1, RecurrenceWeekly, []int{1}, RecurModeFromCompletion},
		{"weekly:mon,wed,fri~", "weekly:mon,wed,fri~", false, 1, RecurrenceWeekly, []int{1, 3, 5}, RecurModeFromCompletion},
		{"2w:mon,fri~", "2w:mon,fri~", false, 2, RecurrenceWeekly, []int{1, 5}, RecurModeFromCompletion},
		{"3w:mon~", "3w:mon~", false, 3, RecurrenceWeekly, []int{1}, RecurModeFromCompletion},

		// Same forms without ~ → mode is empty (FromDue default)
		{"daily no ~", "daily", false, 1, RecurrenceDaily, nil, ""},
		{"2d no ~", "2d", false, 2, RecurrenceDaily, nil, ""},
		{"2w:mon,fri no ~", "2w:mon,fri", false, 2, RecurrenceWeekly, []int{1, 5}, ""},

		// Invalid: bare ~
		{"bare tilde", "~", true, 0, "", nil, ""},
		// Invalid: only ~ after invalid base
		{"invalid~", "invalid~", true, 0, "", nil, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseRecurrence(tc.input)
			if tc.wantNil {
				if got != nil {
					t.Errorf("ParseRecurrence(%q) = %+v, want nil", tc.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseRecurrence(%q) = nil, want non-nil", tc.input)
			}
			if got.Interval != tc.wantInterval {
				t.Errorf("Interval = %d, want %d", got.Interval, tc.wantInterval)
			}
			if got.Unit != tc.wantUnit {
				t.Errorf("Unit = %q, want %q", got.Unit, tc.wantUnit)
			}
			if len(got.Weekdays) != len(tc.wantWeekdays) {
				t.Errorf("Weekdays = %v, want %v", got.Weekdays, tc.wantWeekdays)
			} else {
				for i, wd := range tc.wantWeekdays {
					if got.Weekdays[i] != wd {
						t.Errorf("Weekdays[%d] = %d, want %d", i, got.Weekdays[i], wd)
					}
				}
			}
			if got.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q", got.Mode, tc.wantMode)
			}
		})
	}
}

// TestRecurrencePatternString_Mode verifies that String() appends "~" for
// from-completion mode and does not change output for from-due (default) mode.
func TestRecurrencePatternString_Mode(t *testing.T) {
	tests := []struct {
		name string
		r    *RecurrencePattern
		want string
	}{
		{
			name: "daily from-due",
			r:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			want: "daily",
		},
		{
			name: "daily from-completion",
			r:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily, Mode: RecurModeFromCompletion},
			want: "daily~",
		},
		{
			name: "weekly from-completion",
			r:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Mode: RecurModeFromCompletion},
			want: "weekly~",
		},
		{
			name: "monthly from-completion",
			r:    &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly, Mode: RecurModeFromCompletion},
			want: "monthly~",
		},
		{
			name: "yearly from-completion",
			r:    &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly, Mode: RecurModeFromCompletion},
			want: "yearly~",
		},
		{
			name: "every 3 days from-completion",
			r:    &RecurrencePattern{Interval: 3, Unit: RecurrenceDaily, Mode: RecurModeFromCompletion},
			want: "every 3 days~",
		},
		{
			name: "weekly on Mon from-completion",
			r:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1}, Mode: RecurModeFromCompletion},
			want: "weekly on Mon~",
		},
		{
			name: "every 2 weeks on Mon,Fri from-completion",
			r:    &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly, Weekdays: []int{1, 5}, Mode: RecurModeFromCompletion},
			want: "every 2 weeks on Mon and Fri~",
		},
		{
			name: "nil",
			r:    nil,
			want: "",
		},
		{
			name: "explicit from-due mode is same as default",
			r:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily, Mode: RecurModeFromDue},
			want: "daily",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.r.String()
			if got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRecurrencePatternValidate_Mode verifies that Validate accepts empty and
// known RecurMode values, and rejects unknown ones.
func TestRecurrencePatternValidate_Mode(t *testing.T) {
	base := RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}

	tests := []struct {
		name    string
		mode    RecurMode
		wantErr bool
	}{
		{"empty mode (default)", "", false},
		{"from_due", RecurModeFromDue, false},
		{"from_completion", RecurModeFromCompletion, false},
		{"unknown mode", RecurMode("from_start"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			p.Mode = tc.mode
			err := p.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
