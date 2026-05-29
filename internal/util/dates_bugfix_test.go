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

// TestParseMonthDay_TodayStaysInCurrentYear verifies that a natural month/day
// date equal to today is accepted as today. The documented rule is to advance
// only when the date is already in the past.
func TestParseMonthDay_TodayStaysInCurrentYear(t *testing.T) {
	base := time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)
	got, err := parseMonthDay("may", "29", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(base) {
		t.Errorf("parseMonthDay(may, 29, 2026-05-29) = %s, want %s", got.Format("2006-01-02"), base.Format("2006-01-02"))
	}
}

func TestParseMonthDay_TodayWithTimeStaysInCurrentYear(t *testing.T) {
	base := time.Date(2026, 5, 29, 15, 30, 0, 0, time.UTC)
	got, err := parseMonthDay("may", "29", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseMonthDay(may, 29, 2026-05-29T15:30) = %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// TestParseMonthDay_InvalidLeapDay verifies that "feb 30" remains an error.
func TestParseMonthDay_InvalidLeapDay(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := parseMonthDay("feb", "30", base); err == nil {
		t.Error("expected error for feb 30, got nil")
	}
}

// TestParseMonthDay_LeapDayFromNonLeapBase covers the historical bug where the
// validity check on the current-year candidate rejected "feb 29" outright when
// the base year was not a leap year (Go silently normalised Feb 29 → Mar 1, so
// the month-mismatch guard fired before the leap-year advance loop could run).
// From Jan 1 2025 the next valid Feb 29 is 2028.
func TestParseMonthDay_LeapDayFromNonLeapBase(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err := parseMonthDay("feb", "29", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseMonthDay(feb, 29, 2025-01-01) = %s, want %s", got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// TestParseMonthDay_DayOutOfRange covers calendar months whose maximum day is
// 30 (apr, jun, sep, nov) — "apr 31" must error regardless of base year.
func TestParseMonthDay_DayOutOfRange(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, m := range []string{"apr", "jun", "sep", "nov"} {
		if _, err := parseMonthDay(m, "31", base); err == nil {
			t.Errorf("expected error for %s 31, got nil", m)
		}
	}
	if _, err := parseMonthDay("jan", "0", base); err == nil {
		t.Error("expected error for jan 0, got nil")
	}
	if _, err := parseMonthDay("jan", "32", base); err == nil {
		t.Error("expected error for jan 32, got nil")
	}
}

// TestParseRelativeDate_MultiplierOverflow exercises the second-stage overflow
// that strconv.Atoi cannot catch: values that fit in int64 but blow up when
// multiplied (weeks×7, years×12). Previously the silent overflow wrapped to
// negative and produced wildly wrong dates with no error returned.
func TestParseRelativeDate_MultiplierOverflow(t *testing.T) {
	// 1e18 fits in int64 (~9.22e18 max) but 1e18 * 12 overflows.
	multiplierOverflowInputs := []string{
		"+1000000000000000000w",         // weeks * 7
		"+1000000000000000000m",         // AddMonthsClamped: int(month)+months
		"in 1000000000000000000 weeks",  // weeks * 7
		"in 1000000000000000000 months", // AddMonthsClamped: int(month)+months
		"in 1000000000000000000 years",  // years * 12 first
	}
	for _, in := range multiplierOverflowInputs {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseRelativeDate(in); err == nil {
				t.Errorf("expected error for overflowing input %q, got nil", in)
			}
		})
	}
}

// TestParseFlexibleDate_MultiplierOverflow covers the bare-shorthand path
// for values that overflow under multiplication.
func TestParseFlexibleDate_MultiplierOverflow(t *testing.T) {
	multiplierOverflowInputs := []string{
		"1000000000000000000w",
		"1000000000000000000m",
	}
	for _, in := range multiplierOverflowInputs {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseFlexibleDate(in); err == nil {
				t.Errorf("expected error for overflowing input %q, got nil", in)
			}
		})
	}
}
