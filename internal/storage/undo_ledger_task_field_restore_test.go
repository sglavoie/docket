package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestTaskFieldRestorePreservesSameTaskChangesAcrossRestarts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*models.Task)
		unedit func(*models.Task)
		peer   func(*models.Task, int)
	}{
		{"title", func(task *models.Task) { task.Title = "Edited A" },
			func(task *models.Task) { task.Title = "Original A" },
			func(task *models.Task, step int) { task.Priority = step%3 + 1 }},
		{"priority", func(task *models.Task) { task.Priority = 1 },
			func(task *models.Task) { task.Priority = models.PriorityUnset },
			func(task *models.Task, step int) { task.Title = fmt.Sprintf("Peer title %d", step) }},
		{"context", func(task *models.Task) { task.Context = "office" },
			func(task *models.Task) { task.Context = "" },
			func(task *models.Task, step int) { task.ResumeNote = fmt.Sprintf("Peer resume %d", step) }},
		{"resume_note", func(task *models.Task) { task.ResumeNote = "Continue here" },
			func(task *models.Task) { task.ResumeNote = "" },
			func(task *models.Task, step int) { task.Context = fmt.Sprintf("peer%d", step) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, _ := taskRestoreDocuments()
			after := before.clone()
			tc.edit(after.Tasks[0])
			after.Tasks[0].UpdatedAt = after.Tasks[0].UpdatedAt.Add(time.Minute)
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			for step, redo := range []bool{false, true, false, true} {
				tc.peer(current.Tasks[0], step)
				current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(time.Hour)
				current.Tasks[0].Description = fmt.Sprintf("Peer description %d", step)
				current.Tasks[1].Title = fmt.Sprintf("Other task %d", step)
				writeFile(t, path, mustWriteCurrentDocument(t, current))
				ledger = NewWorkspaceLedger(ledger.dir, ledger.roots)
				var err error
				if redo {
					_, err = ledger.Redo(GlobalScope())
					tc.edit(current.Tasks[0])
				} else {
					_, err = ledger.Undo(GlobalScope())
					tc.unedit(current.Tasks[0])
				}
				if err != nil {
					t.Fatalf("step %d redo=%v: %v", step, redo, err)
				}
				assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
			}
			assertLedgerRestoreStacks(t, ledger, 1, 0)
			assertJournalClean(t, root)
			if _, err := ledger.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTaskFieldRestoreRefusesEachOverlappingField(t *testing.T) {
	for _, redo := range []bool{false, true} {
		for _, field := range []string{"title", "priority", "context", "resume_note"} {
			for _, alreadyDesired := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/redo=%v/already_desired=%v", field, redo, alreadyDesired), func(t *testing.T) {
					ledger, root := newTestLedger(t)
					path := filepath.Join(root, "projects", "alpha", "tasks.md")
					before, after := taskRestoreDocuments()
					// One gesture changes all four fields. Any overlapping field
					// must refuse the whole gesture in either direction.
					after.Tasks[0].Priority = 1
					after.Tasks[0].Context = "office"
					after.Tasks[0].ResumeNote = "Continue here"
					recordTaskRestore(t, ledger, path, before, after)
					writeFile(t, path, mustWriteCurrentDocument(t, after))
					current, desired := after.clone(), before
					if redo {
						if _, err := ledger.Undo(GlobalScope()); err != nil {
							t.Fatal(err)
						}
						current, desired = before.clone(), after
					}
					switch field {
					case "title":
						current.Tasks[0].Title = "Peer title"
						if alreadyDesired {
							current.Tasks[0].Title = desired.Tasks[0].Title
						}
					case "priority":
						current.Tasks[0].Priority = 3
						if alreadyDesired {
							current.Tasks[0].Priority = desired.Tasks[0].Priority
						}
					case "context":
						current.Tasks[0].Context = "peer"
						if alreadyDesired {
							current.Tasks[0].Context = desired.Tasks[0].Context
						}
					case "resume_note":
						current.Tasks[0].ResumeNote = "Peer resume"
						if alreadyDesired {
							current.Tasks[0].ResumeNote = desired.Tasks[0].ResumeNote
						}
					}
					current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(time.Hour)
					content := mustWriteCurrentDocument(t, current)
					writeFile(t, path, content)
					var err error
					if redo {
						_, err = ledger.Redo(GlobalScope())
					} else {
						_, err = ledger.Undo(GlobalScope())
					}
					if !errors.Is(err, ErrStaleArtifact) {
						t.Fatalf("restore error = %v, want ErrStaleArtifact", err)
					}
					assertLedgerRestoreContent(t, path, content)
					assertLedgerRestoreStacks(t, ledger, 0, 0)
					assertJournalClean(t, root)
				})
			}
		}
	}
}

func TestTaskFieldRestoreKeepsWholeTaskChecksForOtherEdits(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*models.Task)
	}{
		{"description", func(task *models.Task) { task.Description = "Local description" }},
		{"tags", func(task *models.Task) { task.Tags = []string{"local"} }},
		{"deadline", func(task *models.Task) { due := task.CreatedAt.AddDate(0, 0, 1); task.DueDate = &due }},
		{"check_in", func(task *models.Task) { day := task.CreatedAt.Truncate(24 * time.Hour); task.NextCheckIn = &day }},
		{"touch_time_only", func(task *models.Task) {}},
	} {
		for _, redo := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/redo=%v", tc.name, redo), func(t *testing.T) {
				ledger, root := newTestLedger(t)
				path := filepath.Join(root, "projects", "alpha", "tasks.md")
				before, _ := taskRestoreDocuments()
				after := before.clone()
				tc.edit(after.Tasks[0])
				if tc.name != "touch_time_only" {
					after.Tasks[0].Title = "Local title beside another edit"
				}
				after.Tasks[0].UpdatedAt = after.Tasks[0].UpdatedAt.Add(time.Minute)
				recordTaskRestore(t, ledger, path, before, after)
				current := after.clone()
				writeFile(t, path, mustWriteCurrentDocument(t, current))
				if redo {
					if _, err := ledger.Undo(GlobalScope()); err != nil {
						t.Fatal(err)
					}
					current = before.clone()
				}
				current.Tasks[0].Priority = 1
				current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(time.Hour)
				content := mustWriteCurrentDocument(t, current)
				writeFile(t, path, content)
				var err error
				if redo {
					_, err = ledger.Redo(GlobalScope())
				} else {
					_, err = ledger.Undo(GlobalScope())
				}
				if !errors.Is(err, ErrStaleArtifact) {
					t.Fatalf("restore error = %v, want ErrStaleArtifact", err)
				}
				assertLedgerRestoreContent(t, path, content)
			})
		}
	}
}

func TestTaskFieldRestorePreservesPeerScheduleAndCollectionsInBulk(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, after := taskRestoreDocuments()
	after.Tasks[1].Title = "Edited B"
	recordTaskRestore(t, ledger, path, before, after)
	current := after.clone()
	day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	for _, task := range current.Tasks {
		task.WorkflowState = models.StateWaiting
		task.StateChangedAt = task.StateChangedAt.Add(time.Hour)
		task.UpdatedAt = task.UpdatedAt.Add(time.Hour)
		task.DueDate = &day
		task.NextCheckIn = &day
		task.CheckInRepeatDays = 7
		task.LastReviewedAt = &day
		task.Comments = []*models.Comment{{Text: "Peer comment", CreatedAt: day}}
		task.Checklist = []models.ChecklistItem{{Text: "Peer item", Done: true}}
		task.Tags = []string{"peer"}
	}
	writeFile(t, path, mustWriteCurrentDocument(t, current))
	for _, redo := range []bool{false, true} {
		var err error
		want := before
		if redo {
			_, err = ledger.Redo(GlobalScope())
			want = after
		} else {
			_, err = ledger.Undo(GlobalScope())
		}
		if err != nil {
			t.Fatal(err)
		}
		for i, task := range current.Tasks {
			task.Title = want.Tasks[i].Title
		}
		assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
	}
}

func TestTaskFieldRestoreReconcilesTouchTimeWithoutFieldDrift(t *testing.T) {
	for _, redo := range []bool{false, true} {
		for _, newer := range []bool{false, true} {
			t.Run(fmt.Sprintf("redo=%v/newer=%v", redo, newer), func(t *testing.T) {
				before, after := taskRestoreDocuments()
				expected, desired := after, before
				if redo {
					expected, desired = before, after
				}
				current := expected.clone()
				current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(30 * time.Second)
				if newer {
					current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(time.Hour)
				}
				stamp := current.Tasks[0].UpdatedAt
				if desired.Tasks[0].UpdatedAt.After(stamp) {
					stamp = desired.Tasks[0].UpdatedAt
				}
				data, ok := rebaseTaskRestore([]byte(mustWriteCurrentDocument(t, expected)),
					[]byte(mustWriteCurrentDocument(t, desired)), []byte(mustWriteCurrentDocument(t, current)), false)
				wanted := desired.clone()
				wanted.Tasks[0].UpdatedAt = stamp
				if !ok || string(data) != mustWriteCurrentDocument(t, wanted) {
					t.Fatalf("touch-only drift restore accepted=%v, got:\n%s", ok, data)
				}
			})
		}
	}
}
