package storage

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/transaction"
)

func TestTaskDeletionRestorePreservesUnrelatedChanges(t *testing.T) {
	for _, change := range []string{"edit", "addition", "deletion", "reorder", "metadata", "empty"} {
		t.Run(change, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			after, before := taskCreationDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			switch change {
			case "edit":
				current.Tasks[0].Title = "Incoming B"
			case "addition":
				child := mergeTestTask("task-d", "B's incoming child")
				child.ParentID = &current.Tasks[0].ID
				current.Tasks = slices.Insert(current.Tasks, 1, child)
			case "deletion":
				current.Tasks = current.Tasks[:1]
			case "reorder":
				slices.Reverse(current.Tasks)
			case "metadata":
				current.ProjectName = "Incoming Alpha"
				current.Comments = []*models.ProjectComment{{ID: "00000000-0000-4000-8000-000000000001", Text: "Peer comment", CreatedAt: current.Tasks[0].CreatedAt}}
			case "empty":
				current.Tasks = nil
			}
			writeFile(t, path, mustWriteCurrentDocument(t, current))
			for _, redo := range []bool{false, true, false, true} {
				ledger = NewWorkspaceLedger(ledger.dir, ledger.roots)
				var err error
				if redo {
					_, err = ledger.Redo(GlobalScope())
				} else {
					_, err = ledger.Undo(GlobalScope())
				}
				if err != nil {
					t.Fatalf("redo=%v: %v", redo, err)
				}
				got, err := parseProjectDocumentContent(mustReadFileString(t, path))
				if err != nil {
					t.Fatal(err)
				}
				restored := indexTasks(got.Tasks)["task-a"]
				if redo && restored != nil || !redo && (restored == nil || !restoreTasksEqual(restored, before.Tasks[1])) {
					t.Fatalf("redo=%v: restored task = %+v", redo, restored)
				}
				unrelated := got.clone()
				unrelated.Tasks = slices.DeleteFunc(unrelated.Tasks, func(task *models.Task) bool { return task.ID == "task-a" })
				if mustWriteCurrentDocument(t, unrelated) != mustWriteCurrentDocument(t, current) {
					t.Fatal("restore changed unrelated tasks, order, or project metadata")
				}
				if !redo && change == "addition" {
					if ids := []string{got.Tasks[0].ID, got.Tasks[1].ID, got.Tasks[2].ID, got.Tasks[3].ID}; !slices.Equal(ids, []string{"task-b", "task-d", "task-a", "task-c"}) {
						t.Fatalf("restore split an incoming subtree: %v", ids)
					}
				}
				current.ProjectName += " later"
				got.ProjectName = current.ProjectName
				writeFile(t, path, mustWriteCurrentDocument(t, got))
			}
			if _, err := ledger.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTaskDeletionRestorePlacement(t *testing.T) {
	for _, position := range []int{0, 1, 2} {
		t.Run(map[int]string{0: "first", 1: "middle", 2: "last"}[position], func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			after, _ := taskCreationDocuments()
			before := after.clone()
			before.Tasks = slices.Insert(before.Tasks, position, mergeTestTask("task-a", "Deleted A"))
			recordTaskRestore(t, ledger, path, before, after)
			after.ProjectName = "Peer's project name"
			writeFile(t, path, mustWriteCurrentDocument(t, after))
			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatal(err)
			}
			before.ProjectName = after.ProjectName
			assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, before))
		})
	}
}

func TestTaskDeletionRestoreRefusesUnsafeChanges(t *testing.T) {
	for _, change := range []string{"identity", "short_id", "title", "description", "touch_time", "missing", "new_child", "adopted_child"} {
		t.Run(change, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			after, before := taskCreationDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			writeFile(t, path, mustWriteCurrentDocument(t, after))
			redo := change != "identity" && change != "short_id"
			current := after.clone()
			if redo {
				if _, err := ledger.Undo(GlobalScope()); err != nil {
					t.Fatal(err)
				}
				current = before.clone()
			}
			switch change {
			case "identity", "short_id":
				incoming := mergeTestTask("task-d", "Incoming task")
				if change == "identity" {
					incoming.ID = before.Tasks[1].ID
				} else {
					incoming.ShortID = before.Tasks[1].ShortID
				}
				current.Tasks = append(current.Tasks, incoming)
			case "title":
				current.Tasks[1].Title = "Incoming A"
			case "description":
				current.Tasks[1].Description = "Incoming description"
			case "touch_time":
				current.Tasks[1].UpdatedAt = current.Tasks[1].UpdatedAt.Add(time.Hour)
			case "missing":
				current.Tasks = slices.Delete(current.Tasks, 1, 2)
			case "new_child":
				child := mergeTestTask("task-d", "Incoming child")
				child.ParentID = &current.Tasks[1].ID
				current.Tasks = append(current.Tasks, child)
			case "adopted_child":
				current.Tasks[2].ParentID = &current.Tasks[1].ID
			}
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

func TestTaskDeletionRestoreKeepsExactChecksForCoupledChanges(t *testing.T) {
	for _, change := range []string{"child", "attachment", "batch", "edit", "metadata", "note", "planning"} {
		t.Run(change, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			after, before := taskCreationDocuments()
			switch change {
			case "child":
				before.Tasks[1].ParentID = &before.Tasks[0].ID
			case "attachment":
				before.Tasks[1].Attachments = []models.Attachment{{Filename: "https://example.com", IsURL: true, AddedAt: before.Tasks[1].CreatedAt}}
			case "batch":
				before.Tasks = append(before.Tasks, mergeTestTask("task-d", "Also deleted"))
			case "edit":
				after.Tasks[0].Title = "Reference cleanup"
			case "metadata":
				after.ProjectName = "Also renamed"
			}
			artifacts := []LedgerArtifact{{Path: path, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)), AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after))}}
			if change == "note" || change == "planning" {
				other := filepath.Join(root, change+".json")
				artifacts = append(artifacts, LedgerArtifact{Path: other, BeforeExists: true, Before: []byte("Before"), AfterExists: true, After: []byte("After")})
				writeFile(t, other, "After")
			}
			recordLedgerRestoreGroup(t, ledger, "deletion", "delete task", artifacts)
			after.ProjectName = "Incoming project name"
			content := mustWriteCurrentDocument(t, after)
			writeFile(t, path, content)
			if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
				t.Fatalf("undo error = %v, want ErrStaleArtifact", err)
			}
			assertLedgerRestoreContent(t, path, content)
			for _, artifact := range artifacts[1:] {
				assertLedgerRestoreContent(t, artifact.Path, "After")
			}
		})
	}
}

func TestTaskDeletionRestoreCrashRecoveryConverges(t *testing.T) {
	for _, redo := range []bool{false, true} {
		for prefix := 0; prefix <= 2; prefix++ {
			t.Run(map[bool]string{false: "undo_", true: "redo_"}[redo]+map[int]string{0: "prepared", 1: "project_written", 2: "ledger_written"}[prefix], func(t *testing.T) {
				ledger, root := newTestLedger(t)
				path := filepath.Join(root, "projects", "alpha", "tasks.md")
				after, before := taskCreationDocuments()
				recordTaskRestore(t, ledger, path, before, after)
				current, desired := after.clone(), before.clone()
				writeFile(t, path, mustWriteCurrentDocument(t, current))
				if redo {
					if _, err := ledger.Undo(GlobalScope()); err != nil {
						t.Fatal(err)
					}
					current, desired = desired, current
				}
				current.Tasks[0].Title, desired.Tasks[0].Title = "Incoming B", "Incoming B"
				writeFile(t, path, mustWriteCurrentDocument(t, current))
				lf, err := ledger.load()
				if err != nil {
					t.Fatal(err)
				}
				entry, idx, ok := selectNewestOwned(lf.Entries, GlobalScope(), "")
				if redo {
					entry, idx, ok = selectNextRedoOwned(lf.RedoEntries, GlobalScope(), "")
				}
				if !ok {
					t.Fatal("no deletion entry")
				}
				journal := transaction.NewJournal(ledger.transactionsDir, ledger.roots, "")
				manifest, err := ledger.prepareRestoreTransaction(journal, lf, entry, idx, redo)
				if err != nil {
					t.Fatal(err)
				}
				if len(manifest.Ops) != 2 {
					t.Fatalf("restore has %d ops, want project and ledger", len(manifest.Ops))
				}
				fixture := ledgerRestoreCrashFixture{root: root, ledger: ledger, transactionsDir: ledger.transactionsDir, manifest: manifest}
				if prefix > 0 {
					manifest.State = transaction.StateCommitting
				}
				writeLedgerRestoreManifest(t, fixture)
				applyLedgerRestoreOpPrefix(t, manifest, prefix)
				if _, err := NewCommitService(ledger.transactionsDir, ledger.roots).RecoverWorkspace(); err != nil {
					t.Fatal(err)
				}
				assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, desired))
				if redo {
					assertLedgerRestoreStacks(t, ledger, 1, 0)
				} else {
					assertLedgerRestoreStacks(t, ledger, 0, 1)
				}
				assertJournalClean(t, root)
			})
		}
	}
}
