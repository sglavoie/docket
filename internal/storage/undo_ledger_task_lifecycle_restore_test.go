package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func lifecycleRestoreDocuments(t *testing.T, from, to models.WorkflowState) (*ProjectDocument, *ProjectDocument) {
	t.Helper()
	before, _ := taskRestoreDocuments()
	task := before.Tasks[0]
	if err := task.ApplyStateTransitionAt(from, task.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !from.IsTerminal() {
		day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
		task.NextCheckIn, task.CheckInRepeatDays = &day, 7
	}
	reviewed := task.CreatedAt
	task.LastReviewedAt = &reviewed
	if from == models.StateBacklog {
		wake := task.UpdatedAt.Add(24 * time.Hour)
		task.DeferredUntil, task.DeferCount = &wake, 2
	}
	after := before.clone()
	if err := after.Tasks[0].ApplyStateTransitionAt(to, task.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return before, after
}

func TestLifecycleRestorePreservesPeerFieldsAcrossRestarts(t *testing.T) {
	for _, state := range []models.WorkflowState{
		models.StateInbox, models.StateBacklog, models.StateDoing, models.StateWaiting,
		models.StatePaused, models.StateSomeday, models.StateDone, models.StateCanceled,
	} {
		from := models.StateBacklog
		if state == from {
			from = models.StateDoing
		}
		t.Run(string(state), func(t *testing.T) {
			testLifecycleRestorePeerFields(t, from, state)
		})
	}
	for _, from := range []models.WorkflowState{models.StateDone, models.StateCanceled} {
		t.Run("reopen_"+string(from), func(t *testing.T) {
			testLifecycleRestorePeerFields(t, from, models.StateInbox)
		})
	}
}

func testLifecycleRestorePeerFields(t *testing.T, from, to models.WorkflowState) {
	t.Helper()
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, after := lifecycleRestoreDocuments(t, from, to)
	recordTaskRestore(t, ledger, path, before, after)
	current := after.clone()
	for step, redo := range []bool{false, true, false, true} {
		task := current.Tasks[0]
		task.Title = fmt.Sprintf("Peer title %d", step)
		task.Priority = step%3 + 1
		task.Description = fmt.Sprintf("Peer description %d", step)
		task.Context, task.ResumeNote = "office", "Peer resume"
		task.Tags = []string{"peer"}
		task.Checklist = []models.ChecklistItem{{Text: "Peer item", Done: true}}
		task.Comments = []*models.Comment{{Text: "Peer comment", CreatedAt: task.CreatedAt}}
		task.Pinned, task.PinnedAt = true, &task.CreatedAt
		task.Review, task.Hidden = true, true
		day := time.Date(2026, 7, 16+step, 0, 0, 0, 0, time.UTC)
		task.DueDate = &day
		task.UpdatedAt = task.UpdatedAt.Add(time.Hour)
		current.Tasks[1].Title = fmt.Sprintf("Other task %d", step)
		writeFile(t, path, mustWriteCurrentDocument(t, current))
		ledger = NewWorkspaceLedger(ledger.dir, ledger.roots)
		var err error
		values := before.Tasks[0]
		if redo {
			_, err = ledger.Redo(GlobalScope())
			values = after.Tasks[0]
		} else {
			_, err = ledger.Undo(GlobalScope())
		}
		if err != nil {
			t.Fatalf("step %d redo=%v: %v", step, redo, err)
		}
		task.WorkflowState, task.StateChangedAt, task.CompletedAt = values.WorkflowState, values.StateChangedAt, values.CompletedAt
		task.DeferredUntil, task.DeferCount = values.DeferredUntil, values.DeferCount
		task.NextCheckIn, task.CheckInRepeatDays = values.NextCheckIn, values.CheckInRepeatDays
		assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
	}
	assertLedgerRestoreStacks(t, ledger, 1, 0)
	assertJournalClean(t, root)
	if _, err := ledger.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleRestoreRefusesOverlappingGroups(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to models.WorkflowState
		peer     func(*models.Task)
	}{
		{"state", models.StateBacklog, models.StateDoing, func(task *models.Task) { task.WorkflowState = models.StateWaiting }},
		{"state_clock", models.StateBacklog, models.StateDoing, func(task *models.Task) { task.StateChangedAt = task.StateChangedAt.Add(time.Second) }},
		{"completion", models.StateBacklog, models.StateDone, func(task *models.Task) {
			stamp := task.UpdatedAt.Add(time.Second)
			task.WorkflowState, task.CompletedAt = models.StateCanceled, &stamp
		}},
		{"deferral", models.StateBacklog, models.StateInbox, func(task *models.Task) {
			wake := task.UpdatedAt.Add(48 * time.Hour)
			task.DeferredUntil = &wake
		}},
		{"deferral_count", models.StateBacklog, models.StateDoing, func(task *models.Task) { task.DeferCount++ }},
		{"check_in", models.StateWaiting, models.StateDone, func(task *models.Task) {
			// Reopen before changing a schedule on a terminal expected image.
			if task.IsComplete() {
				task.WorkflowState, task.CompletedAt = models.StateWaiting, nil
			}
			day := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
			task.NextCheckIn = &day
		}},
		{"check_in_repeat", models.StateWaiting, models.StateDone, func(task *models.Task) {
			if task.IsComplete() {
				task.WorkflowState, task.CompletedAt = models.StateWaiting, nil
			}
			day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
			task.NextCheckIn, task.CheckInRepeatDays = &day, 14
		}},
		{"review_history", models.StateWaiting, models.StateDone, func(task *models.Task) {
			stamp := task.UpdatedAt.Add(time.Second)
			task.LastReviewedAt = &stamp
		}},
	} {
		for _, redo := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/redo=%v", tc.name, redo), func(t *testing.T) {
				ledger, root := newTestLedger(t)
				path := filepath.Join(root, "projects", "alpha", "tasks.md")
				before, after := lifecycleRestoreDocuments(t, tc.from, tc.to)
				recordTaskRestore(t, ledger, path, before, after)
				current := after.clone()
				writeFile(t, path, mustWriteCurrentDocument(t, current))
				if redo {
					if _, err := ledger.Undo(GlobalScope()); err != nil {
						t.Fatal(err)
					}
					current = before.clone()
				}
				tc.peer(current.Tasks[0])
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

func TestLifecycleRestoreKeepsUnchangedCheckInCurrent(t *testing.T) {
	before, after := lifecycleRestoreDocuments(t, models.StateWaiting, models.StateDoing)
	current := after.clone()
	day := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	current.Tasks[0].NextCheckIn, current.Tasks[0].CheckInRepeatDays = &day, 14
	current.Tasks[0].LastReviewedAt = &current.Tasks[0].UpdatedAt
	current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(time.Hour)
	data, ok := rebaseTaskRestore([]byte(mustWriteCurrentDocument(t, after)), []byte(mustWriteCurrentDocument(t, before)), []byte(mustWriteCurrentDocument(t, current)), false)
	current.Tasks[0].WorkflowState, current.Tasks[0].StateChangedAt = before.Tasks[0].WorkflowState, before.Tasks[0].StateChangedAt
	if !ok || string(data) != mustWriteCurrentDocument(t, current) {
		t.Fatalf("restore accepted=%v, got:\n%s", ok, data)
	}
}
