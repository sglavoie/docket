package util

import (
	"errors"
	"testing"
	"time"
)

func mustLoadZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("zone database unavailable for %s: %v", name, err)
	}
	return loc
}

// TestAddCalendarDaysCrossesCalendarBoundaries is the reason this is calendar
// arithmetic rather than a multiple of 24 hours: month lengths, leap days, and
// year ends all have to fall out of shifting the date components.
func TestAddCalendarDaysCrossesCalendarBoundaries(t *testing.T) {
	for name, tc := range map[string]struct {
		from time.Time
		days int
		want string
	}{
		"short month end":        {time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), 1, "2026-02-01"},
		"leap day exists":        {time.Date(2028, 2, 28, 0, 0, 0, 0, time.UTC), 1, "2028-02-29"},
		"leap day skipped":       {time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC), 1, "2026-03-01"},
		"leap year in a year":    {time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC), 365, "2029-02-28"},
		"year end":               {time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), 1, "2027-01-01"},
		"backwards over a month": {time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), -1, "2026-02-28"},
		"a whole ordinary year":  {time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), 365, "2027-03-15"},
		"zero is identity":       {time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), 0, "2026-03-15"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := AddCalendarDays(tc.from, tc.days)
			if err != nil {
				t.Fatalf("AddCalendarDays: %v", err)
			}
			if got.Format(DateFormat) != tc.want {
				t.Fatalf("AddCalendarDays(%s, %d) = %s, want %s",
					tc.from.Format(DateFormat), tc.days, got.Format(DateFormat), tc.want)
			}
		})
	}
}

// TestAddCalendarDaysIsOneDayAcrossDST is the difference between a day and 24
// hours, stated as a test: adding one calendar day over a spring-forward or
// fall-back transition has to land on the next date, which adding 24h to the
// instant would not.
func TestAddCalendarDaysIsOneDayAcrossDST(t *testing.T) {
	newYork := mustLoadZone(t, "America/New_York")
	for name, tc := range map[string]struct {
		from  time.Time
		want  string
		hours float64
	}{
		"spring forward": {time.Date(2026, 3, 8, 0, 0, 0, 0, newYork), "2026-03-09", 23},
		"fall back":      {time.Date(2026, 11, 1, 0, 0, 0, 0, newYork), "2026-11-02", 25},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := AddCalendarDays(tc.from, 1)
			if err != nil {
				t.Fatalf("AddCalendarDays: %v", err)
			}
			if got.Format(DateFormat) != tc.want {
				t.Fatalf("AddCalendarDays = %s, want %s", got.Format(DateFormat), tc.want)
			}
			// The whole point: the elapsed span is not 24 hours, and the answer is
			// right anyway.
			if elapsed := got.Sub(tc.from).Hours(); elapsed != tc.hours {
				t.Fatalf("this day spans %v hours, want %v — the fixture no longer crosses a transition",
					elapsed, tc.hours)
			}
			if DaysBetween(tc.from, got) != 1 {
				t.Fatalf("DaysBetween disagrees that %s is one day after %s", got, tc.from)
			}
		})
	}
}

// TestAddCalendarDaysKeepsItsLocation is what keeps a stored date the date it
// was set to: the shift is computed on the components, but the result belongs to
// the same zone it started in.
func TestAddCalendarDaysKeepsItsLocation(t *testing.T) {
	newYork := mustLoadZone(t, "America/New_York")
	from := time.Date(2026, 3, 15, 0, 0, 0, 0, newYork)
	got, err := AddCalendarDays(from, 7)
	if err != nil {
		t.Fatalf("AddCalendarDays: %v", err)
	}
	if got.Location() != newYork {
		t.Fatalf("location = %s, want %s", got.Location(), newYork)
	}
	if !got.Equal(StartOfDay(got)) {
		t.Fatalf("result %s is not the start of its own day", got)
	}
}

// TestAddCalendarDaysRefusesAnAbsurdOffset keeps the bound shared with every
// other relative-offset reader, so a typo cannot walk a date out of range.
func TestAddCalendarDaysRefusesAnAbsurdOffset(t *testing.T) {
	base := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	for _, days := range []int{maxRelativeOffset + 1, -(maxRelativeOffset + 1)} {
		if _, err := AddCalendarDays(base, days); err == nil {
			t.Fatalf("AddCalendarDays accepted an offset of %d days", days)
		}
	}
}

// TestStartOfCalendarDateRefusesADateTheZoneSkipped is the hazard the whole
// helper exists for: time.Date normalizes a nonexistent date into its neighbour
// and nothing downstream can tell, so a caller that asked for a day the zone has
// none of gets an error rather than a different day.
func TestStartOfCalendarDateRefusesADateTheZoneSkipped(t *testing.T) {
	apia := mustLoadZone(t, "Pacific/Apia")
	if _, err := StartOfCalendarDate(2011, time.December, 30, apia); err == nil {
		t.Fatal("Pacific/Apia accepted 2011-12-30, a date it skipped entirely")
	}
	// The neighbours it does have are still ordinary dates.
	for _, day := range []int{29, 31} {
		if _, err := StartOfCalendarDate(2011, time.December, day, apia); err != nil {
			t.Fatalf("2011-12-%d in Pacific/Apia: %v", day, err)
		}
	}
	// So is the date in a zone that did not skip it.
	if _, err := StartOfCalendarDate(2011, time.December, 30, time.UTC); err != nil {
		t.Fatalf("2011-12-30 in UTC: %v", err)
	}
}

// TestStartOfCalendarDateAcceptsADateWithNoMidnight is the other half of that
// rule, and the case that is easy to get wrong. A zone whose transition happens
// at 00:00 has that date — it just does not start at midnight — so the answer is
// the earliest instant in it rather than an error.
//
// Go resolves a gap in whichever of the two zones it likes: Asia/Beirut answers
// a missing midnight with 01:00 on the date asked for, while America/Havana and
// America/Sao_Paulo answer with 23:00 on the day before. Reading the date
// components off that answer therefore cannot decide whether the date exists,
// which is why the implementation asks at noon instead. Both spellings are
// covered here so a future simplification back to the components fails.
func TestStartOfCalendarDateAcceptsADateWithNoMidnight(t *testing.T) {
	for _, tc := range []struct {
		zone  string
		year  int
		month time.Month
		day   int
	}{
		{"America/Havana", 2026, time.March, 8},
		{"America/Sao_Paulo", 2018, time.November, 4},
		{"Asia/Beirut", 2026, time.March, 29},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			loc := mustLoadZone(t, tc.zone)
			start, err := StartOfCalendarDate(tc.year, tc.month, tc.day, loc)
			if err != nil {
				t.Fatalf("%04d-%02d-%02d in %s: %v", tc.year, tc.month, tc.day, tc.zone, err)
			}
			if year, month, day := start.Date(); year != tc.year || month != tc.month || day != tc.day {
				t.Fatalf("start of date = %s, want %04d-%02d-%02d", start, tc.year, tc.month, tc.day)
			}
			if start.Hour() != 1 {
				t.Skipf("%s no longer skips midnight on that date (hour %d); the rule is unchanged",
					tc.zone, start.Hour())
			}
			// The earliest instant in the date, not merely an instant in it: one
			// second earlier has to fall on the day before.
			if _, _, day := start.Add(-time.Second).Date(); day == tc.day {
				t.Fatalf("%s is not the earliest instant of its date", start)
			}
		})
	}
}

// TestStartOfCalendarDateDefaultsToLocal pins the nil-location contract, since a
// caller passing a zero location means "here" rather than UTC.
func TestStartOfCalendarDateDefaultsToLocal(t *testing.T) {
	start, err := StartOfCalendarDate(2026, time.March, 15, nil)
	if err != nil {
		t.Fatalf("StartOfCalendarDate: %v", err)
	}
	if start.Location() != time.Local {
		t.Fatalf("location = %s, want %s", start.Location(), time.Local)
	}
}

// TestValidateStorableCalendarDateRefusesWhatTheLayoutCannotSpell guards the
// durable document: a date the fixed-width layout renders as something it cannot
// parse back is refused before it is written rather than discovered on load.
func TestValidateStorableCalendarDateRefusesWhatTheLayoutCannotSpell(t *testing.T) {
	for name, value := range map[string]time.Time{
		"ordinary date":      time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		"first storable day": time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		"last storable day":  time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC),
		// Year zero renders four digits and parses back as itself, so it is
		// storable. It is here to record that the rule is round-tripping rather
		// than plausibility: nothing downstream needs the date to be a sane one,
		// only to survive the file.
		"year zero": time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		if err := ValidateStorableCalendarDate(value); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, value := range map[string]time.Time{
		"negative year":   time.Date(-1, 6, 1, 0, 0, 0, 0, time.UTC),
		"five digit year": time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		if err := ValidateStorableCalendarDate(value); err == nil {
			t.Errorf("%s (%s) was accepted as storable", name, value)
		}
	}
}

// TestParseCheckInDateReadsACalendarDay covers the values a check-in surface is
// meant to take. The relative forms resolve against today, so they are checked
// against the same arithmetic rather than against a frozen string.
func TestParseCheckInDateReadsACalendarDay(t *testing.T) {
	got, err := ParseCheckInDate("2026-03-15")
	if err != nil {
		t.Fatalf("ParseCheckInDate: %v", err)
	}
	if got.Format(DateFormat) != "2026-03-15" {
		t.Fatalf("parsed %s, want 2026-03-15", got.Format(DateFormat))
	}
	if !got.Equal(StartOfDay(got)) {
		t.Fatalf("a check-in date carried a time of day: %s", got)
	}
	for _, input := range []string{"today", "tomorrow", "+7d"} {
		day, err := ParseCheckInDate(input)
		if err != nil {
			t.Fatalf("ParseCheckInDate(%q): %v", input, err)
		}
		if !day.Equal(StartOfDay(day)) {
			t.Fatalf("ParseCheckInDate(%q) carried a time of day: %s", input, day)
		}
	}
}

// TestParseCheckInDateResolvesAWrittenDateAsACivilDate is why a written-out
// date does not go through the general parser. Parsing "2026-03-08" in
// America/Havana asks for a midnight that zone does not have, and Go answers
// with 23:00 on the 7th — the right instant under the wrong question, which
// would store a check-in a day early and read back as a value nobody typed.
func TestParseCheckInDateResolvesAWrittenDateAsACivilDate(t *testing.T) {
	havana := mustLoadZone(t, "America/Havana")
	previous := time.Local
	time.Local = havana
	defer func() { time.Local = previous }()

	got, err := ParseCheckInDate("2026-03-08")
	if err != nil {
		t.Fatalf("ParseCheckInDate: %v", err)
	}
	if got.Format(DateFormat) != "2026-03-08" {
		t.Fatalf("parsed 2026-03-08 in %s as %s", havana, got)
	}
	if !IsStartOfCalendarDate(got) {
		t.Fatalf("%s is not the start of its own calendar date", got)
	}
	// A date the zone has none of is refused rather than shifted onto a neighbour.
	apiaPrevious := time.Local
	time.Local = mustLoadZone(t, "Pacific/Apia")
	defer func() { time.Local = apiaPrevious }()
	if _, err := ParseCheckInDate("2011-12-30"); err == nil {
		t.Fatal("Pacific/Apia accepted a date it skipped entirely")
	}
}

// TestParseCheckInDateRefusesATimeOfDay is the field's defining refusal, and it
// names itself: a check-in has nowhere to put an hour, so a caller that typed
// one is told which of the two date fields it is setting instead of getting a
// format error.
func TestParseCheckInDateRefusesATimeOfDay(t *testing.T) {
	for _, input := range []string{
		"2026-03-15 09:00",
		"tomorrow 15:30",
		// Explicit midnight resolves to the same instant the bare date does, and is
		// still refused: accepting it would mean the value read back is not the
		// value that was typed.
		"2026-03-15 00:00",
	} {
		if _, err := ParseCheckInDate(input); !errors.Is(err, ErrCheckInsAreDateOnly) {
			t.Errorf("ParseCheckInDate(%q) error = %v, want ErrCheckInsAreDateOnly", input, err)
		}
	}
}

// TestParseCheckInDateRefusesTheHourForm keeps "+Nh" defer-only. It is a
// separate refusal from the clock suffix because it is a different mistake, and
// the message inherited from ParseCalendarDate says so.
func TestParseCheckInDateRefusesTheHourForm(t *testing.T) {
	for _, input := range []string{"+4h", "+48h"} {
		_, err := ParseCheckInDate(input)
		if err == nil {
			t.Errorf("ParseCheckInDate(%q) was accepted", input)
			continue
		}
		if errors.Is(err, ErrCheckInsAreDateOnly) {
			t.Errorf("ParseCheckInDate(%q) blamed the clock suffix: %v", input, err)
		}
	}
}

// TestParseCheckInDateRefusesNonsense covers the ordinary unparseable values, so
// the field is not quietly permissive about text it cannot resolve.
func TestParseCheckInDateRefusesNonsense(t *testing.T) {
	for _, input := range []string{"", "   ", "someday", "2026-13-01", "2026-02-30", "15/03/2026"} {
		if _, err := ParseCheckInDate(input); err == nil {
			t.Errorf("ParseCheckInDate(%q) was accepted", input)
		}
	}
}
