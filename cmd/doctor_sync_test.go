package cmd

import (
	"testing"
	"time"
)

// TestDoctorClockPairLayoutShowsTheDifference pins the one choice the state
// clock report makes about precision: two instants inside one second print in
// full, so the report never reads as the same time twice.
func TestDoctorClockPairLayoutShowsTheDifference(t *testing.T) {
	base := time.Date(2026, 9, 27, 0, 5, 20, 140920954, time.UTC)
	cases := []struct {
		name string
		a, b time.Time
		want string
	}{
		{"same second", base, base.Add(-4 * time.Microsecond), time.RFC3339Nano},
		{"different seconds", base, base.Add(-3 * time.Second), time.RFC3339},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			layout := doctorClockPairLayout(tc.a, tc.b)
			if layout != tc.want {
				t.Fatalf("layout = %q, want %q", layout, tc.want)
			}
			if tc.a.Format(layout) == tc.b.Format(layout) {
				t.Fatalf("both instants print as %q", tc.a.Format(layout))
			}
		})
	}
}
