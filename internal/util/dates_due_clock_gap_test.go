package util

import (
	"strings"
	"testing"
	"time"
)

func TestParseDueDateRefusesSkippedClockTimes(t *testing.T) {
	for _, tc := range []struct {
		zone, input string
	}{
		{"America/New_York", "2026-03-08 02:30"},
		{"Australia/Lord_Howe", "2026-10-04 02:15"},
		{"America/Havana", "2026-03-08 00:30"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			loc := mustLoadZone(t, tc.zone)
			saved := time.Local
			time.Local = loc
			t.Cleanup(func() { time.Local = saved })
			got, _, err := ParseDueDate(tc.input)
			if err == nil {
				t.Fatalf("accepted nonexistent %s as %s", tc.input, got)
			}
			if !strings.Contains(err.Error(), tc.input) || !strings.Contains(err.Error(), tc.zone) {
				t.Fatalf("refusal must name the requested clock and zone: %v", err)
			}
			// Valid clocks on either side of the gap still round-trip exactly.
			for _, clock := range []string{"01:15", "03:15"} {
				input := strings.Fields(tc.input)[0] + " " + clock
				got, hasTime, err := ParseDueDate(input)
				if err != nil || !hasTime || got.Format("2006-01-02 15:04") != input {
					t.Errorf("%q = %s, hasTime=%t, err=%v", input, got, hasTime, err)
				}
			}
		})
	}
}
