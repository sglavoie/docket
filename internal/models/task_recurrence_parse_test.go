package models

import (
	"testing"
)

func TestParseRecurrence(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *RecurrencePattern
		wantNil bool
	}{
		// Keywords
		{"daily keyword", "daily", &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}, false},
		{"weekly keyword", "weekly", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}, false},
		{"biweekly keyword", "biweekly", &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}, false},
		{"monthly keyword", "monthly", &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}, false},
		{"yearly keyword", "yearly", &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}, false},

		// Interval format
		{"1d", "1d", &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}, false},
		{"2d", "2d", &RecurrencePattern{Interval: 2, Unit: RecurrenceDaily}, false},
		{"3w", "3w", &RecurrencePattern{Interval: 3, Unit: RecurrenceWeekly}, false},
		{"1m", "1m", &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}, false},
		{"2m", "2m", &RecurrencePattern{Interval: 2, Unit: RecurrenceMonthly}, false},
		{"1y", "1y", &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}, false},
		{"2y", "2y", &RecurrencePattern{Interval: 2, Unit: RecurrenceYearly}, false},
		{"10d", "10d", &RecurrencePattern{Interval: 10, Unit: RecurrenceDaily}, false},

		// Weekly with days
		{"weekly:mon", "weekly:mon", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1}}, false},
		{"weekly:mon,wed,fri", "weekly:mon,wed,fri", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1, 3, 5}}, false},
		{"2w:mon,fri", "2w:mon,fri", &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly, Weekdays: []int{1, 5}}, false},
		{"weekly:sunday", "weekly:sunday", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{0}}, false},
		{"weekly:tue,thu", "weekly:tue,thu", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{2, 4}}, false},

		// Invalid input
		{"empty string", "", nil, true},
		{"invalid keyword", "invalid", nil, true},
		{"0d", "0d", nil, true},
		{"negative -1d", "-1d", nil, true},
		{"letters only", "abc", nil, true},
		{"no number", "d", nil, true},
		{"invalid unit", "2x", nil, true},
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
				t.Fatalf("ParseRecurrence(%q) = nil, want %+v", tc.input, tc.want)
			}
			if got.Interval != tc.want.Interval {
				t.Errorf("ParseRecurrence(%q).Interval = %d, want %d", tc.input, got.Interval, tc.want.Interval)
			}
			if got.Unit != tc.want.Unit {
				t.Errorf("ParseRecurrence(%q).Unit = %s, want %s", tc.input, got.Unit, tc.want.Unit)
			}
			if len(got.Weekdays) != len(tc.want.Weekdays) {
				t.Errorf("ParseRecurrence(%q).Weekdays = %v, want %v", tc.input, got.Weekdays, tc.want.Weekdays)
			} else {
				for i, wd := range got.Weekdays {
					if wd != tc.want.Weekdays[i] {
						t.Errorf("ParseRecurrence(%q).Weekdays[%d] = %d, want %d", tc.input, i, wd, tc.want.Weekdays[i])
					}
				}
			}
		})
	}
}

func TestParseRecurrenceCaseSensitive(t *testing.T) {
	// Document that ParseRecurrence is case-sensitive for keywords
	tests := []struct {
		input   string
		wantNil bool
	}{
		{"DAILY", true},
		{"Daily", true},
		{"WEEKLY", true},
		{"Weekly", true},
		{"MONTHLY", true},
		{"Monthly", true},
		{"YEARLY", true},
		{"Yearly", true},
		{"BIWEEKLY", true},
		{"Biweekly", true},
		{"2D", true},     // uppercase unit
		{"3W", true},     // uppercase unit
		{"1M", true},     // uppercase unit
		{"2Y", true},     // uppercase unit
		{"daily", false}, // lowercase works
		{"2d", false},    // lowercase unit works
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseRecurrence(tc.input)
			if tc.wantNil && got != nil {
				t.Errorf("ParseRecurrence(%q) = %+v, want nil (case-sensitive)", tc.input, got)
			}
			if !tc.wantNil && got == nil {
				t.Errorf("ParseRecurrence(%q) = nil, want non-nil", tc.input)
			}
		})
	}
}

func TestParseRecurrenceMoreEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantInt int
	}{
		{"large interval 999d", "999d", false, 999},
		{"large interval 100y", "100y", false, 100},
		{"at max interval", "3650d", false, 3650},
		{"over max interval", "3651d", true, 0},
		{"leading space", " daily", true, 0},
		{"trailing space", "daily ", true, 0},
		{"colon with no days", "1w:", true, 0},
		{"weekly colon no days", "weekly:", true, 0},
		{"just colon", ":", true, 0},
		{"number only", "123", true, 0},
		{"very long number", "99999999d", true, 0},
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
			if got.Interval != tc.wantInt {
				t.Errorf("ParseRecurrence(%q).Interval = %d, want %d", tc.input, got.Interval, tc.wantInt)
			}
		})
	}
}

// TestParseRecurrenceOverflowGuard verifies that extremely large interval strings
// are rejected without panicking, even when they would overflow int before the
// MaxRecurrenceInterval check.
func TestParseRecurrenceOverflowGuard(t *testing.T) {
	inputs := []string{
		"99999999999999d",       // 14 digits — would overflow before MaxRecurrenceInterval check
		"99999999999999999999d", // 20 digits — would definitely overflow int64
		"999999999999999w",
	}
	for _, s := range inputs {
		t.Run(s, func(t *testing.T) {
			// Must not panic and must return nil.
			got := ParseRecurrence(s)
			if got != nil {
				t.Errorf("ParseRecurrence(%q) = %+v, want nil", s, got)
			}
		})
	}
}

func TestParseWeekdays(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"mon", []int{1}},
		{"mon,wed,fri", []int{1, 3, 5}},
		{"sunday,saturday", []int{0, 6}},
		{"tue, thu", []int{2, 4}}, // with spaces
		{"MON,WED", []int{1, 3}},  // uppercase
		{"Mon,Mon", []int{1}},     // duplicates removed
		{"invalid", []int{}},
		{"", []int{}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := parseWeekdays(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("parseWeekdays(%q) = %v, want %v", tc.input, got, tc.want)
				return
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("parseWeekdays(%q)[%d] = %d, want %d", tc.input, i, v, tc.want[i])
				}
			}
		})
	}
}

func TestParseWeekdaysEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []int
	}{
		{"mixed valid/invalid", "mon,invalid,fri", []int{1, 5}},
		{"extra whitespace", "mon , wed,  fri", []int{1, 3, 5}},
		{"tabs", "mon\t,\twed", []int{1, 3}},
		{"leading/trailing commas", ",mon,wed,", []int{1, 3}},
		{"multiple commas", "mon,,wed", []int{1, 3}},
		{"all invalid", "foo,bar,baz", []int{}},
		{"all full names", "monday,tuesday,wednesday", []int{1, 2, 3}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseWeekdays(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("parseWeekdays(%q) = %v, want %v", tc.input, got, tc.want)
				return
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("parseWeekdays(%q)[%d] = %d, want %d", tc.input, i, v, tc.want[i])
				}
			}
		})
	}
}
