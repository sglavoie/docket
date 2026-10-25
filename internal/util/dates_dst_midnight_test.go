package util

import (
	"testing"
	"time"
)

// A zone whose daylight-saving change happens at 00:00 has no midnight on that
// date: America/Havana starts 2026-03-08 at 01:00 and America/Santiago starts
// 2026-09-06 there. Stepping a local day-start with AddDate lands on 23:00 the
// day before, or carries the 01:00 onto the following date, where DueHasTime
// reads it as a time of day. Every relative due/defer form must answer the
// start of the calendar date it names, in the caller's zone, and nothing else.
func TestRelativeDatesLandOnTheStartOfTheirDateWhereMidnightIsSkipped(t *testing.T) {
	cases := []struct {
		zone   string
		now    string
		want   string
		inputs []string
	}{
		// Havana, Saturday before the gap: the gap day itself.
		{"America/Havana", "2026-03-07 10:00", "2026-03-08", []string{"tomorrow", "tmrw", "+1d", "in 1 day", "sun", "this sun", "eow"}},
		{"America/Havana", "2026-03-01 10:00", "2026-03-08", []string{"+7d", "+1w", "next week", "in 1 week", "next sun", "mar 8", "8 mar"}},
		{"America/Havana", "2026-02-08 10:00", "2026-03-08", []string{"+1m", "next month", "in 1 month"}},
		// Havana, on the gap day (which starts at 01:00): days after it.
		{"America/Havana", "2026-03-08 12:00", "2026-03-08", []string{"today", "eod", "+0d"}},
		{"America/Havana", "2026-03-08 12:00", "2026-03-09", []string{"tomorrow", "+1d", "mon", "in 1 day"}},
		{"America/Havana", "2026-03-08 12:00", "2026-03-15", []string{"+1w", "next week", "+7d", "sun"}},
		{"America/Havana", "2026-03-08 12:00", "2026-04-08", []string{"+1m", "next month"}},
		{"America/Havana", "2026-03-08 12:00", "2026-03-07", []string{"yesterday"}},
		{"America/Havana", "2026-03-09 12:00", "2026-03-08", []string{"yesterday", "ytd"}},
		// Santiago, Saturday before its gap, and on it.
		{"America/Santiago", "2026-09-05 18:00", "2026-09-06", []string{"tomorrow", "+1d", "sun", "in 1 day"}},
		{"America/Santiago", "2026-08-30 09:00", "2026-09-06", []string{"+1w", "next week", "next sun", "sep 6"}},
		{"America/Santiago", "2026-09-06 09:00", "2026-09-07", []string{"tomorrow", "mon", "+1d"}},
		{"America/Santiago", "2026-09-06 09:00", "2026-09-13", []string{"+1w", "next week"}},
	}
	for _, tc := range cases {
		loc := mustLoadZone(t, tc.zone)
		now, err := time.ParseInLocation("2006-01-02 15:04", tc.now, loc)
		if err != nil {
			t.Fatalf("parse now %q: %v", tc.now, err)
		}
		for _, input := range tc.inputs {
			got, err := ParseRelativeDateWithConfigAt(input, "sun", now)
			if err != nil {
				t.Errorf("%s from %s: %q: %v", tc.zone, tc.now, input, err)
				continue
			}
			if got.Location() != loc {
				t.Errorf("%s from %s: %q: location %v, want %v", tc.zone, tc.now, input, got.Location(), loc)
			}
			if day := got.Format(DateFormat); day != tc.want {
				t.Errorf("%s from %s: %q = %s, want date %s", tc.zone, tc.now, input, got, tc.want)
			}
			if !IsStartOfCalendarDate(got) {
				t.Errorf("%s from %s: %q = %s, which carries a time of day", tc.zone, tc.now, input, got)
			}
		}
	}
}

// The defer surface takes the same day forms, and its hour form stays a real
// elapsed duration rather than becoming calendar arithmetic.
func TestDeferInstantKeepsDaysCalendarAndHoursElapsedWhereMidnightIsSkipped(t *testing.T) {
	loc := mustLoadZone(t, "America/Havana")
	now := time.Date(2026, time.March, 7, 22, 30, 0, 0, loc)

	day, err := ParseDeferInstantAt("+1d", now)
	if err != nil {
		t.Fatalf("+1d: %v", err)
	}
	if day.Format(DateFormat) != "2026-03-08" || !IsStartOfCalendarDate(day) {
		t.Fatalf("+1d = %s, want the 01:00 start of 2026-03-08", day)
	}

	hours, err := ParseDeferInstantAt("+3h", now)
	if err != nil {
		t.Fatalf("+3h: %v", err)
	}
	if want := now.Add(3 * time.Hour); !hours.Equal(want) {
		t.Fatalf("+3h = %s, want %s (three elapsed hours)", hours, want)
	}
}

// A written date on the gap day resolves to that date, not to 23:00 the day
// before, and a due value without a clock stays date-only.
func TestWrittenDateOnASkippedMidnightIsThatDate(t *testing.T) {
	loc := mustLoadZone(t, "America/Havana")
	saved := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = saved })

	for _, input := range []string{"2026-03-08", "03/08/2026", "08.03.2026"} {
		got, err := ParseRelativeDate(input)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		if got.Format(DateFormat) != "2026-03-08" || !IsStartOfCalendarDate(got) {
			t.Errorf("%q = %s, want the start of 2026-03-08", input, got)
		}
	}

	due, hasTime, err := ParseDueDate("2026-03-08")
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if hasTime || due.Format(DateFormat) != "2026-03-08" || !IsStartOfCalendarDate(due) {
		t.Fatalf("due = %s hasTime=%v, want date-only 2026-03-08", due, hasTime)
	}
	timed, hasTime, err := ParseDueDate("2026-03-08 15:30")
	if err != nil {
		t.Fatalf("timed due: %v", err)
	}
	if !hasTime || timed.Format("2006-01-02 15:04") != "2026-03-08 15:30" {
		t.Fatalf("timed due = %s hasTime=%v, want 2026-03-08 15:30", timed, hasTime)
	}
}

func TestNextWeekdayAfterLandsOnTheStartOfItsDateWhereMidnightIsSkipped(t *testing.T) {
	cases := []struct {
		zone    string
		now     time.Time
		weekday time.Weekday
		want    string
	}{
		{"America/Havana", time.Date(2026, time.March, 7, 9, 0, 0, 0, time.UTC), time.Sunday, "2026-03-08"},
		{"America/Havana", time.Date(2026, time.March, 8, 15, 0, 0, 0, time.UTC), time.Monday, "2026-03-09"},
		{"America/Havana", time.Date(2026, time.March, 8, 15, 0, 0, 0, time.UTC), time.Saturday, "2026-03-14"},
		{"America/Santiago", time.Date(2026, time.September, 5, 15, 0, 0, 0, time.UTC), time.Sunday, "2026-09-06"},
		{"America/Santiago", time.Date(2026, time.September, 6, 15, 0, 0, 0, time.UTC), time.Monday, "2026-09-07"},
	}
	for _, tc := range cases {
		loc := mustLoadZone(t, tc.zone)
		now := tc.now.In(loc)
		got := NextWeekdayAfter(now, tc.weekday)
		if got.Format(DateFormat) != tc.want || !IsStartOfCalendarDate(got) {
			t.Errorf("%s NextWeekdayAfter(%s, %s) = %s, want the start of %s", tc.zone, now, tc.weekday, got, tc.want)
		}
	}
}

func TestStartOfCalendarDaysAfter(t *testing.T) {
	havana := mustLoadZone(t, "America/Havana")
	sat := time.Date(2026, time.March, 7, 10, 0, 0, 0, havana)
	sun := time.Date(2026, time.March, 8, 10, 0, 0, 0, havana)
	cases := []struct {
		from time.Time
		days int
		want string
	}{
		{sat, 0, "2026-03-07"},
		{sat, 1, "2026-03-08"},
		{sun, 0, "2026-03-08"},
		{sun, 1, "2026-03-09"},
		{sun, 7, "2026-03-15"},
		{sun, -1, "2026-03-07"},
		{sun, 30, "2026-04-07"},
	}
	for _, tc := range cases {
		got := StartOfCalendarDaysAfter(tc.from, tc.days)
		if got.Format(DateFormat) != tc.want || !IsStartOfCalendarDate(got) {
			t.Errorf("StartOfCalendarDaysAfter(%s, %d) = %s, want the start of %s", tc.from, tc.days, got, tc.want)
		}
	}

	// Pacific/Apia has no 2011-12-30 at all; the fallback answers the
	// surviving neighbor rather than an instant inside no date.
	apia := mustLoadZone(t, "Pacific/Apia")
	from := time.Date(2011, time.December, 29, 10, 0, 0, 0, apia)
	if got := StartOfCalendarDaysAfter(from, 1); got.Format(DateFormat) != "2011-12-31" || !IsStartOfCalendarDate(got) {
		t.Errorf("Apia skip = %s, want the start of 2011-12-31", got)
	}
	// An offset beyond AddCalendarDays' range still answers a day start.
	if got := StartOfCalendarDaysAfter(sat, 7_000_000); !IsStartOfCalendarDate(got) {
		t.Errorf("large offset = %s, want a day start", got)
	}
}
