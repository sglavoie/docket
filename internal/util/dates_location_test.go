package util

import (
	"testing"
	"time"
)

func TestWrittenDatesUseTheSuppliedClockLocation(t *testing.T) {
	saved := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = saved })

	loc := mustLoadZone(t, "America/Havana")
	now := time.Date(2026, time.March, 7, 10, 0, 0, 0, loc)
	want := time.Date(2026, time.March, 8, 1, 0, 0, 0, loc)
	parsers := map[string]func(string, time.Time) (time.Time, error){
		"calendar": ParseCalendarDateAt,
		"defer":    ParseDeferInstantAt,
		"relative": func(input string, now time.Time) (time.Time, error) {
			return ParseRelativeDateWithConfigAt(input, "sun", now)
		},
	}
	for name, parse := range parsers {
		t.Run(name, func(t *testing.T) {
			for _, input := range []string{"tomorrow", "2026-03-08", "03/08/2026", "08.03.2026"} {
				got, err := parse(input, now)
				if err != nil {
					t.Fatalf("%q: %v", input, err)
				}
				if !got.Equal(want) || got.Location() != loc {
					t.Errorf("%q = %s (%s), want %s (%s)", input, got, got.Location(), want, loc)
				}
			}
			apia := mustLoadZone(t, "Pacific/Apia")
			beforeSkippedDate := time.Date(2011, time.December, 29, 10, 0, 0, 0, apia)
			for _, input := range []string{"tomorrow", "2011-12-30", "12/30/2011", "30.12.2011"} {
				if got, err := parse(input, beforeSkippedDate); err == nil {
					t.Errorf("%q accepted nonexistent local date as %s", input, got)
				}
			}
		})
	}
}
