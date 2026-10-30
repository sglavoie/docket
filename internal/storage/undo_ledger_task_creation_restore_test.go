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

func taskCreationDocuments() (before, after *ProjectDocument) {
	before = mergeTestDoc("Alpha", mergeTestTask("task-b", "Original B"), mergeTestTask("task-c", "Original C"))
	after = before.clone()
	after.Tasks = slices.Insert(after.Tasks, 1, mergeTestTask("task-a", "Created A"))
	return before, after
}

func TestTaskCreationRestorePlacement(t *testing.T) {
	for _, position := range []int{-1, 0, 1, 2} {
		t.Run(map[int]string{-1: "empty", 0: "first", 1: "middle", 2: "last"}[position], func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, _ := taskCreationDocuments()
			index := position
			if position == -1 {
				before.Tasks, index = nil, 0
			}
			after := before.clone()
			after.Tasks = slices.Insert(after.Tasks, index, mergeTestTask("task-a", "Created A"))
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			current.ProjectName = "Incoming Alpha"
			writeFile(t, path, mustWriteCurrentDocument(t, current))
			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatal(err)
			}
			before.ProjectName = current.ProjectName
			assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, before))
			if position == 1 {
				// A peer can add a child of B while A is undone. Redo must
				// keep that subtree together and retain its new contents.
				child := mergeTestTask("task-d", "B's incoming child")
				child.ParentID = &before.Tasks[0].ID
				before.Tasks = slices.Insert(before.Tasks, 1, child)
				current.Tasks = slices.Insert(current.Tasks, 1, child)
				writeFile(t, path, mustWriteCurrentDocument(t, before))
			}
			if _, err := ledger.Redo(GlobalScope()); err != nil {
				t.Fatal(err)
			}
			assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
		})
	}
}

func TestTaskCreationRestoreCrashRecoveryConverges(t *testing.T) {
	for prefix := 0; prefix <= 2; prefix++ {
		t.Run(map[int]string{0: "prepared", 1: "project_written", 2: "ledger_written"}[prefix], func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := taskCreationDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			current.Tasks[0].Title = "Incoming B"
			writeFile(t, path, mustWriteCurrentDocument(t, current))
			lf, err := ledger.load()
			if err != nil {
				t.Fatal(err)
			}
			entry, idx, ok := selectNewestOwned(lf.Entries, GlobalScope(), "")
			if !ok {
				t.Fatal("no creation entry")
			}
			journal := transaction.NewJournal(ledger.transactionsDir, ledger.roots, "")
			manifest, err := ledger.prepareRestoreTransaction(journal, lf, entry, idx, false, nil)
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
			service := NewCommitService(ledger.transactionsDir, ledger.roots)
			if _, err := service.RecoverWorkspace(); err != nil {
				t.Fatal(err)
			}
			before.Tasks[0].Title = "Incoming B"
			assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, before))
			assertLedgerRestoreStacks(t, ledger, 0, 1)
			if _, err := ledger.Redo(GlobalScope()); err != nil {
				t.Fatal(err)
			}
			assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
			assertJournalClean(t, root)
		})
	}
}

func TestTaskCreationRestorePreservesUnrelatedChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ProjectDocument)
	}{
		{"edit", func(doc *ProjectDocument) { doc.Tasks[0].Title = "Incoming B" }},
		{"addition", func(doc *ProjectDocument) { doc.Tasks = append(doc.Tasks, mergeTestTask("task-d", "Incoming D")) }},
		{"deletion", func(doc *ProjectDocument) { doc.Tasks = doc.Tasks[1:] }},
		{"reorder", func(doc *ProjectDocument) { doc.Tasks[0], doc.Tasks[2] = doc.Tasks[2], doc.Tasks[0] }},
		{"no_original_neighbors", func(doc *ProjectDocument) { doc.Tasks = doc.Tasks[1:2] }},
		{"project_metadata", func(doc *ProjectDocument) {
			doc.ProjectName = "Incoming name"
			doc.Comments = []*models.ProjectComment{{ID: "00000000-0000-4000-8000-000000000001", Text: "Incoming comment", CreatedAt: doc.Tasks[0].CreatedAt}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := taskCreationDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			tc.change(current)
			writeFile(t, path, mustWriteCurrentDocument(t, current))
			// Repeat across restarts; peer work can arrive on either stack.
			for _, redo := range []bool{false, true, false, true} {
				ledger = NewWorkspaceLedger(ledger.dir, ledger.roots)
				if redo {
					if _, err := ledger.Redo(GlobalScope()); err != nil {
						t.Fatal(err)
					}
				} else if _, err := ledger.Undo(GlobalScope()); err != nil {
					t.Fatal(err)
				}
				got, err := parseProjectDocumentContent(mustReadFileString(t, path))
				if err != nil {
					t.Fatal(err)
				}
				created := indexTasks(got.Tasks)["task-a"]
				if redo && (created == nil || !restoreTasksEqual(created, after.Tasks[1])) || !redo && created != nil {
					t.Fatalf("redo=%v: created task = %+v", redo, created)
				}
				withoutCreated := func(doc *ProjectDocument) *ProjectDocument {
					result := doc.clone()
					result.Tasks = slices.DeleteFunc(result.Tasks, func(task *models.Task) bool { return task.ID == "task-a" })
					return result
				}
				if mustWriteCurrentDocument(t, withoutCreated(got)) != mustWriteCurrentDocument(t, withoutCreated(current)) {
					t.Fatal("restore changed unrelated tasks, their order, or project metadata")
				}
				current = got
				current.ProjectName += " later"
				writeFile(t, path, mustWriteCurrentDocument(t, current))
			}
			if _, err := ledger.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTaskCreationRestoreRefusesUnsafeUndo(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ProjectDocument)
	}{
		{"title", func(doc *ProjectDocument) { doc.Tasks[1].Title = "Incoming A" }},
		{"other_field", func(doc *ProjectDocument) { doc.Tasks[1].Description = "Incoming description" }},
		{"touch_time", func(doc *ProjectDocument) { doc.Tasks[1].UpdatedAt = doc.Tasks[1].UpdatedAt.Add(time.Hour) }},
		{"missing", func(doc *ProjectDocument) { doc.Tasks = slices.Delete(doc.Tasks, 1, 2) }},
		{"new_child", func(doc *ProjectDocument) {
			child := mergeTestTask("task-d", "Incoming child")
			child.ParentID = &doc.Tasks[1].ID
			doc.Tasks = append(doc.Tasks, child)
		}},
		{"adopted_child", func(doc *ProjectDocument) { doc.Tasks[2].ParentID = &doc.Tasks[1].ID }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := taskCreationDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			tc.change(current)
			content := mustWriteCurrentDocument(t, current)
			writeFile(t, path, content)
			if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
				t.Fatalf("undo error = %v, want ErrStaleArtifact", err)
			}
			assertLedgerRestoreContent(t, path, content)
			assertLedgerRestoreStacks(t, ledger, 0, 0)
			assertJournalClean(t, root)
		})
	}
}

func TestTaskCreationRestoreRefusesRedoIdentityCollisions(t *testing.T) {
	for _, sameID := range []bool{false, true} {
		t.Run(map[bool]string{false: "short_id", true: "task_id"}[sameID], func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := taskCreationDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			writeFile(t, path, mustWriteCurrentDocument(t, after))
			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatal(err)
			}
			current := before.clone()
			incoming := mergeTestTask("task-d", "Incoming task")
			if sameID {
				incoming.ID = after.Tasks[1].ID
			} else {
				incoming.ShortID = after.Tasks[1].ShortID
			}
			current.Tasks = append(current.Tasks, incoming)
			content := mustWriteCurrentDocument(t, current)
			writeFile(t, path, content)
			if _, err := ledger.Redo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
				t.Fatalf("redo error = %v, want ErrStaleArtifact", err)
			}
			assertLedgerRestoreContent(t, path, content)
			assertLedgerRestoreStacks(t, ledger, 0, 0)
		})
	}
}

func TestTaskCreationRestoreKeepsExactChecksForCoupledChanges(t *testing.T) {
	for _, name := range []string{"child", "attachment", "batch", "edit", "project_metadata", "note", "deletion_with_edit"} {
		t.Run(name, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := taskCreationDocuments()
			switch name {
			case "child":
				after.Tasks[1].ParentID = &after.Tasks[0].ID
			case "attachment":
				after.Tasks[1].Attachments = []models.Attachment{{Filename: "https://example.com", IsURL: true, AddedAt: after.Tasks[1].CreatedAt}}
			case "batch":
				after.Tasks = append(after.Tasks, mergeTestTask("task-d", "Also created"))
			case "edit":
				after.Tasks[0].Title = "Also edited"
			case "project_metadata":
				after.ProjectName = "Also renamed"
			case "deletion_with_edit":
				before, after = after, before
				after.Tasks[0].Title = "Also edited"
			}
			artifacts := []LedgerArtifact{{Path: path, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)), AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after))}}
			if name == "note" {
				note := filepath.Join(root, "projects", "alpha", "notes", "note.md")
				artifacts = append(artifacts, LedgerArtifact{Path: note, BeforeExists: true, Before: []byte("Before"), AfterExists: true, After: []byte("After")})
				writeFile(t, note, "After")
			}
			recordLedgerRestoreGroup(t, ledger, "coupled", "capture", artifacts)
			current := after.clone()
			current.ProjectName = "Incoming project name"
			content := mustWriteCurrentDocument(t, current)
			writeFile(t, path, content)
			if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
				t.Fatalf("undo error = %v, want ErrStaleArtifact", err)
			}
			assertLedgerRestoreContent(t, path, content)
			if name == "note" {
				assertLedgerRestoreContent(t, artifacts[1].Path, "After")
			}
		})
	}
}
