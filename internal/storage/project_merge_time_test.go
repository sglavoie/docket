package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestDeadlineMergeUsesStoredCalendarAndClock(t *testing.T) {
	west := time.FixedZone("west", -10*3600)
	east := time.FixedZone("east", 14*3600)
	due := func(day, hour int, zone *time.Location) *time.Time {
		value := time.Date(2026, 9, day, hour, 30, 0, 0, zone)
		if hour == 0 {
			value = time.Date(2026, 9, day, 0, 0, 0, 0, zone)
		}
		return &value
	}
	for _, tc := range []struct {
		name                 string
		base, local, current *time.Time
		deleted, conflict    bool
		want                 *time.Time
	}{
		{"same new date", nil, due(1, 0, west), due(1, 0, east), false, false, due(1, 0, west)},
		{"unopposed date edit", due(1, 0, west), due(1, 0, east), due(2, 0, west), false, false, due(2, 0, west)},
		{"delete unchanged date", due(1, 0, west), nil, due(1, 0, east), true, false, nil},
		{"different dates at same instant", nil, due(1, 0, west), due(2, 0, east), false, true, nil},
		{"same new clock", nil, due(1, 9, west), due(1, 9, east), false, false, due(1, 9, west)},
		{"unopposed clock edit", due(1, 9, west), due(1, 9, east), due(1, 10, west), false, false, due(1, 10, west)},
		{"delete unchanged clock", due(1, 9, west), nil, due(1, 9, east), true, false, nil},
		{"different clocks at same instant", nil, due(1, 9, west), due(2, 9, east), false, true, nil},
	} {
		for _, swap := range []bool{false, true} {
			name := tc.name
			if swap {
				name += "/reversed sides"
			}
			t.Run(name, func(t *testing.T) {
				doc := func(date *time.Time) *ProjectDocument {
					task := mergeTestTask("a", "Task")
					task.DueDate = date
					return mergeTestDoc("p", task)
				}
				base, local, current := doc(tc.base), doc(tc.local), doc(tc.current)
				if tc.deleted {
					local = mergeTestDoc("p")
				}
				if swap {
					local, current = current, local
				}
				two := mustMerge(t, base, local, current)
				many, conflicts := mustReconcile(t, base, local, current)
				if two.HasConflicts() != tc.conflict || (len(conflicts) > 0) != tc.conflict {
					t.Fatalf("merge conflicts = %+v, reconcile conflicts = %+v, want conflict=%t", two.Conflicts, conflicts, tc.conflict)
				}
				if tc.conflict {
					for _, conflict := range append(two.Conflicts, conflicts...) {
						if conflict.Field != "due_date" || conflict.Local == conflict.Current {
							t.Fatalf("deadline conflict does not distinguish the stored values: %+v", conflict)
						}
						wantLocal, wantCurrent := tc.local, tc.current
						if swap {
							wantLocal, wantCurrent = wantCurrent, wantLocal
						}
						layout := "2006-01-02"
						if wantLocal.Hour() != 0 {
							layout += " 15:04"
						}
						if conflict.Local != wantLocal.Format(layout) || conflict.Current != wantCurrent.Format(layout) {
							t.Fatalf("deadline conflict spells the wrong dates or clocks: %+v", conflict)
						}
					}
					return
				}
				for _, output := range []*ProjectDocument{two.Merged, many} {
					if tc.deleted {
						if len(output.Tasks) != 0 {
							t.Fatal("unchanged deadline prevented deletion")
						}
						continue
					}
					if got := output.Tasks[0].DueDate; got == nil || got.Format("2006-01-02 15:04") != tc.want.Format("2006-01-02 15:04") {
						t.Fatalf("deadline = %v, want %v", got, tc.want)
					}
				}
			})
		}
	}
}

func TestTaskInstantMergeIgnoresTimeRepresentation(t *testing.T) {
	stamp := time.Date(2026, 9, 1, 12, 0, 0, 123, time.UTC)
	device := time.FixedZone("device", -6*3600)
	for _, field := range []struct {
		name string
		set  func(*models.Task, time.Time)
		get  func(*models.Task) *time.Time
	}{
		{"created_at", func(task *models.Task, at time.Time) { task.CreatedAt = at }, func(task *models.Task) *time.Time { return &task.CreatedAt }},
		{"state_changed_at", func(task *models.Task, at time.Time) { task.StateChangedAt = at }, func(task *models.Task) *time.Time { return &task.StateChangedAt }},
		{"completed_at", func(task *models.Task, at time.Time) {
			task.WorkflowState = models.StateDone
			task.CompletedAt = &at
		}, func(task *models.Task) *time.Time { return task.CompletedAt }},
		{"pinned_at", func(task *models.Task, at time.Time) { task.Pinned = true; task.PinnedAt = &at }, func(task *models.Task) *time.Time { return task.PinnedAt }},
		{"deferred_until", func(task *models.Task, at time.Time) { task.DeferredUntil = &at }, func(task *models.Task) *time.Time { return task.DeferredUntil }},
	} {
		for _, tc := range []struct {
			name              string
			local, current    time.Time
			deleted, conflict bool
			want              time.Time
		}{
			{"delete unchanged", stamp, stamp.In(device), true, false, stamp},
			{"delete edited", stamp, stamp.In(device).Add(time.Nanosecond), true, true, stamp},
			{"unopposed edit", stamp.In(device), stamp.Add(time.Nanosecond), false, false, stamp.Add(time.Nanosecond)},
			{"identical edits", stamp.Add(time.Nanosecond), stamp.In(device).Add(time.Nanosecond), false, false, stamp.Add(time.Nanosecond)},
			{"divergent edits", stamp.Add(time.Nanosecond), stamp.In(device).Add(2 * time.Nanosecond), false, true, stamp},
		} {
			for _, swap := range []bool{false, true} {
				name := field.name + "/" + tc.name
				if swap {
					name += "/reversed sides"
				}
				t.Run(name, func(t *testing.T) {
					doc := func(at time.Time) *ProjectDocument {
						task := mergeTestTask("a", "Task")
						field.set(task, at)
						return mergeTestDoc("p", task)
					}
					base, local, current := doc(stamp), doc(tc.local), doc(tc.current)
					if tc.deleted {
						local = mergeTestDoc("p")
					}
					if swap {
						local, current = current, local
					}
					two := mustMerge(t, base, local, current)
					many, conflicts := mustReconcile(t, base, local, current)
					if two.HasConflicts() != tc.conflict || (len(conflicts) > 0) != tc.conflict {
						t.Fatalf("merge conflicts = %+v, reconcile conflicts = %+v, want conflict=%t", two.Conflicts, conflicts, tc.conflict)
					}
					if tc.conflict {
						return
					}
					for _, output := range []*ProjectDocument{two.Merged, many} {
						if tc.deleted {
							if len(output.Tasks) != 0 {
								t.Fatal("unchanged instant prevented deletion")
							}
							continue
						}
						if got := field.get(output.Tasks[0]); got == nil || !got.Equal(tc.want) {
							t.Fatalf("merged %s = %v, want %v", field.name, got, tc.want)
						}
					}
				})
			}
		}
	}
}

func TestDeadlineMergeDateWithoutMidnight(t *testing.T) {
	havana, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatal(err)
	}
	for _, clock := range []string{"", "01:00"} {
		t.Run("UTC clock "+clock, func(t *testing.T) {
			doc := func(zone *time.Location, clock string) *ProjectDocument {
				task := mergeTestTask("a", "Task")
				task.DueDate, err = parseProjectDocumentDueDate("2026-03-08", clock, zone)
				if err != nil {
					t.Fatal(err)
				}
				return mergeTestDoc("p", task)
			}
			base := mergeTestDoc("p", mergeTestTask("a", "Task"))
			local, current := doc(havana, ""), doc(time.UTC, clock)
			two := mustMerge(t, base, local, current)
			many, conflicts := mustReconcile(t, base, local, current)
			wantConflict := clock != ""
			if two.HasConflicts() != wantConflict || (len(conflicts) > 0) != wantConflict {
				t.Fatalf("merge conflicts = %+v, reconcile conflicts = %+v, want conflict=%t", two.Conflicts, conflicts, wantConflict)
			}
			if wantConflict {
				return
			}
			for _, output := range []*ProjectDocument{two.Merged, many} {
				record, dueTime := currentTaskBaseFieldsFromModel(output.Tasks[0])
				if record.DueDate != "2026-03-08" || dueTime != "" {
					t.Fatalf("date-only deadline became %q with clock %q", record.DueDate, dueTime)
				}
			}
		})
	}
}
