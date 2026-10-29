package storage

import (
	"testing"
	"time"
)

func TestDeletionIgnoresCheckInTimeRepresentation(t *testing.T) {
	for _, changed := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			name := "same instant"
			if changed {
				name = "changed instant"
			}
			if swap {
				name += "/reversed sides"
			}
			t.Run(name, func(t *testing.T) {
				reviewed := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
				base := mergeTestDoc("p", checkInMergeTask(t, "Task", checkInMergeDay(t, 9, 1), 7, &reviewed))
				other := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
				// Serialization may change a time's location without changing its instant.
				inOtherZone := reviewed.In(time.FixedZone("device", -6*60*60))
				if changed {
					inOtherZone = inOtherZone.Add(time.Second)
				}
				other.Tasks[0].LastReviewedAt = &inOtherZone
				deleted := mergeTestDoc("p")
				local, current := deleted, other
				if swap {
					local, current = current, local
				}
				result := mustMerge(t, base, local, current)
				if result.HasConflicts() != changed {
					t.Fatalf("merge conflicts = %+v, want conflict=%t", result.Conflicts, changed)
				}
				if !changed && len(result.Merged.Tasks) != 0 {
					t.Fatal("unchanged task survived deletion")
				}
				merged, conflicts := mustReconcile(t, base, local, current)
				if (len(conflicts) > 0) != changed {
					t.Fatalf("reconcile conflicts = %+v, want conflict=%t", conflicts, changed)
				}
				if (len(merged.Tasks) > 0) != changed {
					t.Fatalf("reconcile kept %d tasks, want kept=%t", len(merged.Tasks), changed)
				}
			})
		}
	}
}
