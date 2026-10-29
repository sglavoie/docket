package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestTaskNestedInstantsMergeByValue(t *testing.T) {
	stamp := time.Date(2026, 9, 1, 12, 0, 0, 123, time.UTC)
	zone := time.FixedZone("device", -6*3600)
	for _, field := range []string{"comments", "attachments"} {
		for _, tc := range []struct {
			name              string
			local, current    time.Time
			deleted, conflict bool
			want              time.Time
		}{
			{"delete unchanged", stamp, stamp.In(zone), true, false, stamp},
			{"delete edited", stamp, stamp.In(zone).Add(time.Nanosecond), true, true, stamp},
			{"unopposed edit", stamp.In(zone), stamp.Add(time.Nanosecond), false, false, stamp.Add(time.Nanosecond)},
			{"identical edits", stamp.Add(time.Nanosecond), stamp.In(zone).Add(time.Nanosecond), false, false, stamp.Add(time.Nanosecond)},
			{"divergent edits", stamp.Add(time.Nanosecond), stamp.In(zone).Add(2 * time.Nanosecond), false, true, stamp},
		} {
			for _, swap := range []bool{false, true} {
				name := field + "/" + tc.name
				if swap {
					name += "/reversed sides"
				}
				t.Run(name, func(t *testing.T) {
					doc := func(at time.Time) *ProjectDocument {
						task := mergeTestTask("a", "Task")
						if field == "comments" {
							task.Comments = []*models.Comment{{Text: "Comment", CreatedAt: at}}
						} else {
							task.Attachments = []models.Attachment{{Filename: "https://example.com", IsURL: true, OriginalPath: "https://example.com", AddedAt: at}}
						}
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
						t.Fatalf("merge conflicts = %+v, reconciliation conflicts = %+v, want conflict=%t", two.Conflicts, conflicts, tc.conflict)
					}
					if tc.conflict {
						return
					}
					for _, output := range []*ProjectDocument{two.Merged, many} {
						if tc.deleted {
							if len(output.Tasks) != 0 {
								t.Fatal("timestamp representation prevented deletion")
							}
							continue
						}
						got := output.Tasks[0].Comments
						at := output.Tasks[0].Attachments
						var actual time.Time
						if field == "comments" {
							actual = got[0].CreatedAt
						} else {
							actual = at[0].AddedAt
						}
						if !actual.Equal(tc.want) {
							t.Fatalf("merged instant = %v, want %v", actual, tc.want)
						}
					}
				})
			}
		}
	}
}

func TestTaskNestedInstantEqualityKeepsContentConflicts(t *testing.T) {
	for _, field := range []string{"comments", "attachments"} {
		t.Run(field, func(t *testing.T) {
			doc := func(text string) *ProjectDocument {
				task := mergeTestTask("a", "Task")
				if field == "comments" {
					task.Comments = []*models.Comment{{Text: text, CreatedAt: task.CreatedAt}}
				} else {
					task.Attachments = []models.Attachment{{Filename: text, AddedAt: task.CreatedAt}}
				}
				return mergeTestDoc("p", task)
			}
			base, local, current := doc("base"), doc("local"), doc("current")
			two := mustMerge(t, base, local, current)
			_, conflicts := mustReconcile(t, base, local, current)
			if !two.HasConflicts() || len(conflicts) == 0 {
				t.Fatal("divergent content must still conflict")
			}
		})
	}
}
