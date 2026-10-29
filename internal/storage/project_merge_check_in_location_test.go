package storage

import (
	"testing"
	"time"
)

func TestCheckInMergeComparesCalendarDatesAcrossZones(t *testing.T) {
	west := time.FixedZone("west", -10*3600)
	east := time.FixedZone("east", 14*3600)
	day := func(d int, zone *time.Location) *time.Time {
		value := time.Date(2026, time.September, d, 0, 0, 0, 0, zone)
		return &value
	}
	for _, tc := range []struct {
		name                  string
		base, local, current  *time.Time
		deleted, wantConflict bool
		wantDay               int
	}{
		{"identical new schedules", nil, day(1, west), day(1, east), false, false, 1},
		{"unopposed reschedule", day(1, west), day(1, east), day(2, west), false, false, 2},
		{"delete versus unchanged date", day(1, west), nil, day(1, east), true, false, 0},
		{"different dates sharing an instant", nil, day(1, west), day(2, east), false, true, 0},
	} {
		for _, swap := range []bool{false, true} {
			name := tc.name
			if swap {
				name += "/reversed sides"
			}
			t.Run(name, func(t *testing.T) {
				doc := func(next *time.Time) *ProjectDocument {
					return mergeTestDoc("p", checkInMergeTask(t, "Task", next, 0, nil))
				}
				base, local, current := doc(tc.base), doc(tc.local), doc(tc.current)
				if tc.deleted {
					local = mergeTestDoc("p")
				}
				if swap {
					local, current = current, local
				}
				result := mustMerge(t, base, local, current)
				if result.HasConflicts() != tc.wantConflict {
					t.Fatalf("merge conflicts = %+v, want conflict=%t", result.Conflicts, tc.wantConflict)
				}
				merged, conflicts := mustReconcile(t, base, local, current)
				if (len(conflicts) > 0) != tc.wantConflict {
					t.Fatalf("reconcile conflicts = %+v, want conflict=%t", conflicts, tc.wantConflict)
				}
				if tc.wantConflict {
					return
				}
				for _, output := range []*ProjectDocument{result.Merged, merged} {
					if tc.deleted {
						if len(output.Tasks) != 0 {
							t.Fatal("unchanged schedule prevented deletion")
						}
					} else if got := output.Tasks[0].NextCheckIn.Day(); got != tc.wantDay {
						t.Fatalf("merged check-in day = %d, want %d", got, tc.wantDay)
					}
				}
			})
		}
	}
}
