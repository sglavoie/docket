package storage

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/transaction"
)

func taskRestoreDocuments() (before, after *ProjectDocument) {
	before = mergeTestDoc("Alpha", mergeTestTask("task-a", "Original A"), mergeTestTask("task-b", "Original B"))
	after = before.clone()
	after.Tasks[0].Title = "Edited A"
	after.Tasks[0].UpdatedAt = after.Tasks[0].UpdatedAt.Add(time.Minute)
	return before, after
}

func recordTaskRestore(t *testing.T, ledger *WorkspaceLedger, path string, before, after *ProjectDocument) {
	t.Helper()
	recordLedgerRestoreGroup(t, ledger, "task-edit", "edit task", []LedgerArtifact{{
		Path: path, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)),
		AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after)),
	}})
}

func TestTaskRestorePreservesUnrelatedChangesThroughUndoRedo(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ProjectDocument)
	}{
		{"edit", func(doc *ProjectDocument) {
			doc.Tasks[1].Title = "Incoming B"
			doc.Tasks[1].UpdatedAt = doc.Tasks[1].UpdatedAt.Add(time.Hour)
		}},
		{"addition", func(doc *ProjectDocument) { doc.Tasks = append(doc.Tasks, mergeTestTask("task-c", "Incoming C")) }},
		{"deletion", func(doc *ProjectDocument) { doc.Tasks = doc.Tasks[:1] }},
		{"reordering", func(doc *ProjectDocument) { doc.Tasks[0], doc.Tasks[1] = doc.Tasks[1], doc.Tasks[0] }},
		{"project_metadata", func(doc *ProjectDocument) {
			doc.ProjectName = "Renamed Alpha"
			doc.Comments = []*models.ProjectComment{{
				ID: "00000000-0000-4000-8000-000000000001", Text: "Incoming project comment", CreatedAt: doc.Tasks[0].CreatedAt,
			}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := taskRestoreDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			tc.change(current)
			writeFile(t, path, mustWriteCurrentDocument(t, current))
			for _, redo := range []bool{false, true, false, true} {
				// Reopening proves the result does not depend on an in-memory
				// merge base or a rewritten copy of the original ledger images.
				ledger = NewWorkspaceLedger(ledger.dir, ledger.roots)
				var err error
				wanted := current.clone()
				if redo {
					_, err = ledger.Redo(GlobalScope())
				} else {
					_, err = ledger.Undo(GlobalScope())
					for i, task := range wanted.Tasks {
						if task.ID == before.Tasks[0].ID {
							wanted.Tasks[i] = before.Tasks[0]
						}
					}
				}
				if err != nil {
					t.Fatalf("restore redo=%v: %v", redo, err)
				}
				assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, wanted))
			}
			if _, err := ledger.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTaskRestoreRefusesOverlappingChanges(t *testing.T) {
	for _, redo := range []bool{false, true} {
		for _, tc := range []struct {
			name   string
			change func(*ProjectDocument)
		}{
			{"same_field", func(doc *ProjectDocument) { doc.Tasks[0].Title = "Incoming A" }},
			{"missing_task", func(doc *ProjectDocument) { doc.Tasks = doc.Tasks[1:] }},
		} {
			t.Run(tc.name+map[bool]string{false: "_undo", true: "_redo"}[redo], func(t *testing.T) {
				ledger, root := newTestLedger(t)
				path := filepath.Join(root, "projects", "alpha", "tasks.md")
				before, after := taskRestoreDocuments()
				recordTaskRestore(t, ledger, path, before, after)
				current := after.clone()
				writeFile(t, path, mustWriteCurrentDocument(t, current))
				if redo {
					if _, err := ledger.Undo(GlobalScope()); err != nil {
						t.Fatal(err)
					}
					current = before.clone()
				}
				tc.change(current)
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
			})
		}
	}
}

func TestTaskRestoreRechecksChangesBeforeRedo(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, after := taskRestoreDocuments()
	recordTaskRestore(t, ledger, path, before, after)
	current := after.clone()
	current.Tasks[1].Title = "First incoming B"
	writeFile(t, path, mustWriteCurrentDocument(t, current))
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatal(err)
	}
	current.Tasks[0] = before.Tasks[0]
	current.Tasks[1].Title = "Second incoming B"
	writeFile(t, path, mustWriteCurrentDocument(t, current))
	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatal(err)
	}
	current.Tasks[0] = after.Tasks[0]
	assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
}

func TestTaskRestoreRefusesInvalidCombinedHierarchy(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, _ := taskRestoreDocuments()
	before.Tasks[0].WorkflowState = models.StateDone
	stamp := before.Tasks[0].UpdatedAt
	before.Tasks[0].CompletedAt = &stamp
	after := before.clone()
	after.Tasks[0].WorkflowState = models.StateBacklog
	after.Tasks[0].CompletedAt = nil
	recordTaskRestore(t, ledger, path, before, after)
	current := after.clone()
	parent := current.Tasks[0].ID
	current.Tasks[1].ParentID = &parent
	current.Tasks[0].Title = "Incoming A"
	content := mustWriteCurrentDocument(t, current)
	writeFile(t, path, content)
	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("undo error = %v, want refusal to finish a parent with an open child", err)
	}
	assertLedgerRestoreContent(t, path, content)
}

func TestTaskRestorePreflightsEveryArtifactBeforeWriting(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	note := filepath.Join(root, "projects", "alpha", "notes", "note.md")
	before, after := taskRestoreDocuments()
	recordLedgerRestoreGroup(t, ledger, "mixed-edit", "edit task and note", []LedgerArtifact{
		{Path: path, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)),
			AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after))},
		{Path: note, BeforeExists: true, Before: []byte("Before"), AfterExists: true, After: []byte("After")},
	})
	current := after.clone()
	current.Tasks[0].Priority = 1
	current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(time.Hour)
	current.Tasks[1].Title = "Incoming B"
	content := mustWriteCurrentDocument(t, current)
	writeFile(t, path, content)
	writeFile(t, note, "Incoming note")
	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("undo error = %v, want ErrStaleArtifact", err)
	}
	assertLedgerRestoreContent(t, path, content)
	assertLedgerRestoreContent(t, note, "Incoming note")
	assertJournalClean(t, root)
}

func TestTaskRestoreKeepsExactChecksForOtherOperations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ProjectDocument)
	}{
		{"create_task", func(doc *ProjectDocument) { doc.Tasks = append(doc.Tasks, mergeTestTask("task-c", "New C")) }},
		{"delete_task", func(doc *ProjectDocument) { doc.Tasks = doc.Tasks[:1] }},
		{"reorder", func(doc *ProjectDocument) { doc.Tasks[0], doc.Tasks[1] = doc.Tasks[1], doc.Tasks[0] }},
		{"project_name", func(doc *ProjectDocument) { doc.ProjectName = "Renamed Alpha" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := taskRestoreDocuments()
			tc.change(after)
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			current.ProjectName = "Incoming name"
			content := mustWriteCurrentDocument(t, current)
			writeFile(t, path, content)
			if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
				t.Fatalf("undo error = %v, want ErrStaleArtifact", err)
			}
			assertLedgerRestoreContent(t, path, content)
		})
	}
}

func TestTaskRestoreDoesNotRebaseNotesOrAttachments(t *testing.T) {
	for _, relative := range []string{
		"projects/alpha/notes/tasks.md",
		"projects/alpha/.attachments/task-a/tasks.md",
		"projects/alpha/notes/projects/beta/tasks.md",
	} {
		t.Run(relative, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, filepath.FromSlash(relative))
			before, after := taskRestoreDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			current.Tasks[0].Priority = 1
			current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(time.Hour)
			current.Tasks[1].Title = "Incoming B"
			content := mustWriteCurrentDocument(t, current)
			writeFile(t, path, content)
			if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
				t.Fatalf("undo error = %v, want ErrStaleArtifact", err)
			}
			assertLedgerRestoreContent(t, path, content)
		})
	}
}

func TestTaskRestoreKeepsScopedEntriesBehindNewerLocalWrites(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, after := taskRestoreDocuments()
	note := filepath.Join(root, "projects", "alpha", "notes", "note.md")
	recordLedgerRestoreGroup(t, ledger, "task-edit", "edit task and note", []LedgerArtifact{
		{Path: path, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)),
			AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after))},
		{Path: note, BeforeExists: true, Before: []byte("Before"), AfterExists: true, After: []byte("After")},
	})
	writeFile(t, note, "After")
	current := after.clone()
	current.Tasks[1].Title = "Newer local B"
	content := mustWriteCurrentDocument(t, current)
	writeFile(t, path, content)
	// Artifact scopes can select a grouped entry behind another local entry
	// that wrote the same project. It must remain blocked, not retired or
	// semantically restored out of order.
	recordLedgerRestoreGroup(t, ledger, "newer-local", "edit B", []LedgerArtifact{{
		Path: path, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, after)), AfterExists: true, After: []byte(content),
	}})
	if _, err := ledger.Undo(LedgerScope{ArtifactRoot: note}); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("scoped undo error = %v, want ErrStaleArtifact", err)
	}
	assertLedgerRestoreContent(t, path, content)
	assertLedgerRestoreStacks(t, ledger, 2, 0)
}

func TestTaskRestoreWalksMultipleLocalEntries(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, after := taskRestoreDocuments()
	recordTaskRestore(t, ledger, path, before, after)
	later := after.clone()
	later.Tasks[0].Title = "Later A"
	later.Tasks[0].UpdatedAt = later.Tasks[0].UpdatedAt.Add(time.Minute)
	recordLedgerRestoreGroup(t, ledger, "later-edit", "edit A again", []LedgerArtifact{{
		Path: path, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, after)),
		AfterExists: true, After: []byte(mustWriteCurrentDocument(t, later)),
	}})
	current := later.clone()
	current.Tasks[1].Title = "Incoming B"
	writeFile(t, path, mustWriteCurrentDocument(t, current))
	for _, step := range []struct {
		redo bool
		want *ProjectDocument
	}{{false, after}, {false, before}, {true, after}, {true, later}} {
		var err error
		if step.redo {
			_, err = ledger.Redo(GlobalScope())
		} else {
			_, err = ledger.Undo(GlobalScope())
		}
		if err != nil {
			t.Fatal(err)
		}
		current.Tasks[0] = step.want.Tasks[0]
		assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
	}
	assertLedgerRestoreStacks(t, ledger, 2, 0)
}

func TestTaskRestoreBulkEditRefusesChangedMember(t *testing.T) {
	ledger, root := newTestLedger(t)
	path := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, after := taskRestoreDocuments()
	after.Tasks[1].Title = "Edited B"
	recordTaskRestore(t, ledger, path, before, after)
	current := after.clone()
	current.Tasks[1].Title = "Incoming title on a bulk target"
	content := mustWriteCurrentDocument(t, current)
	writeFile(t, path, content)
	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("bulk undo error = %v, want ErrStaleArtifact", err)
	}
	assertLedgerRestoreContent(t, path, content)
}

func TestTaskRestoreCrashRecoveryConverges(t *testing.T) {
	// One rebased project write and the ledger movement share one manifest.
	// Interrupt it before and after each operation, recover, and then Redo.
	for prefix := 0; prefix <= 2; prefix++ {
		t.Run(map[int]string{0: "prepared", 1: "project_written", 2: "ledger_written"}[prefix], func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := taskRestoreDocuments()
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			current.Tasks[0].Priority = 1
			current.Tasks[0].UpdatedAt = current.Tasks[0].UpdatedAt.Add(time.Hour)
			current.Tasks[1].Title = "Incoming B"
			writeFile(t, path, mustWriteCurrentDocument(t, current))
			lf, err := ledger.load()
			if err != nil {
				t.Fatal(err)
			}
			entry, idx, ok := selectNewestOwned(lf.Entries, GlobalScope(), "")
			if !ok {
				t.Fatal("no undo entry")
			}
			journal := transaction.NewJournal(ledger.transactionsDir, ledger.roots, "")
			manifest, err := ledger.prepareRestoreTransaction(journal, lf, entry, idx, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(manifest.Ops) != 2 {
				t.Fatalf("restore manifest has %d operations, want project and ledger", len(manifest.Ops))
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
			current.Tasks[0].Title = before.Tasks[0].Title
			assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
			assertLedgerRestoreStacks(t, ledger, 0, 1)
			if _, err := ledger.Redo(GlobalScope()); err != nil {
				t.Fatal(err)
			}
			current.Tasks[0].Title = after.Tasks[0].Title
			assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, current))
			assertJournalClean(t, root)
		})
	}
}
