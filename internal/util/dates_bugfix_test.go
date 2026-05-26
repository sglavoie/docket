package util

import (
	"strings"
	"testing"
	"time"
)

// TestParseRelativeDate_NextWeekday verifies "next <weekday>" semantics:
// it should always land on the named weekday in next ISO week, i.e. the
// occurrence between 7 and 13 days from today (inclusive). The previous
// implementation added an extra +7 on top of an already future-skipping
// helper, so on the same weekday "next mon" produced today+14 instead of
// today+7.
func TestParseRelativeDate_NextWeekday(t *testing.T) {
	today := StartOfDay(time.Now())
	for _, name := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		t.Run("next "+name, func(t *testing.T) {
			got, err := ParseRelativeDate("next " + name)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			diff := int(got.Sub(today).Hours() / 24)
			if diff < 7 || diff > 13 {
				t.Errorf("next %s = today+%d days, want 7..13", name, diff)
			}
			target := weekdays[name]
			if got.Weekday() != target {
				t.Errorf("next %s landed on %s, want %s", name, got.Weekday(), target)
			}
		})
	}
}

// TestParseRelativeDate_IntegerOverflow verifies that absurdly large numeric
// inputs produce an error rather than silently producing the wrong date by
// ignoring strconv.Atoi's overflow.
func TestParseRelativeDate_IntegerOverflow(t *testing.T) {
	overflowInputs := []string{
		"+99999999999999999999d",
		"+99999999999999999999w",
		"+99999999999999999999m",
		"in 99999999999999999999 days",
		"in 99999999999999999999 weeks",
		"in 99999999999999999999 months",
		"in 99999999999999999999 years",
	}
	for _, in := range overflowInputs {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseRelativeDate(in); err == nil {
				t.Errorf("expected error for overflowing input %q, got nil", in)
			}
		})
	}
}

// TestParseFlexibleDate_IntegerOverflow covers the bare-shorthand path
// "Nd/Nw/Nm" which uses its own strconv.Atoi call.
func TestParseFlexibleDate_IntegerOverflow(t *testing.T) {
	overflowInputs := []string{
		"99999999999999999999d",
		"99999999999999999999w",
		"99999999999999999999m",
	}
	for _, in := range overflowInputs {
		t.Run(in, func(t *testing.T) {
			_, err := ParseFlexibleDate(in)
			if err == nil {
				t.Errorf("expected error for overflowing input %q, got nil", in)
				return
			}
			// Either the bare-shorthand fallback rejects it, or the underlying
			// ParseRelativeDate rejects it; both surface a non-nil error.
			if !strings.Contains(strings.ToLower(err.Error()), "invalid") &&
				!strings.Contains(strings.ToLower(err.Error()), "unrecognized") {
				t.Errorf("error %q does not mention invalid/unrecognized", err.Error())
			}
		})
	}
}

// TestParseMonthDay_LeapDayAdvancesToNextLeapYear is a deterministic regression
// test for the silent-march-1 bug: parsing "feb 29" from a base date of
// 2024-03-01 (after the leap day has passed) must resolve to 2028-02-29,
// not 2025-03-01.
func TestParseMonthDay_LeapDayAdvancesToNextLeapYear(t *testing.T) {
	// Base date: one day after the 2024 leap day, so 2024 is in the past.
	base := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	got, err := parseMonthDay("feb", "29", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseMonthDay(feb, 29, 2024-03-01) = %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// TestParseMonthDay_LeapDayInCurrentLeapYear verifies that "feb 29" resolves
// to the current year when the current year is a leap year and the day is
// still in the future.
func TestParseMonthDay_LeapDayInCurrentLeapYear(t *testing.T) {
	// Base date: February 1, 2028 (a leap year), before Feb 29.
	base := time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC)
	got, err := parseMonthDay("feb", "29", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseMonthDay(feb, 29, 2028-02-01) = %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// TestParseMonthDay_InvalidLeapDay verifies that "feb 30" remains an error.
func TestParseMonthDay_InvalidLeapDay(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := parseMonthDay("feb", "30", base); err == nil {
		t.Error("expected error for feb 30, got nil")
	}
}
