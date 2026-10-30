package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func commentRestoreDocuments(action string) (before, after *ProjectDocument) {
	before, _ = taskRestoreDocuments()
	before.Comments = []*models.ProjectComment{
		{ID: "00000000-0000-4000-8000-000000000001", Text: "Original", CreatedAt: before.Tasks[0].CreatedAt},
		{ID: "00000000-0000-4000-8000-000000000002", Text: "Other", CreatedAt: before.Tasks[0].CreatedAt.Add(time.Minute)},
	}
	after = before.clone()
	switch action {
	case "add":
		before.Comments = before.Comments[1:]
	case "edit":
		after.Comments[0].Text = "Edited"
	case "delete":
		after.Comments = after.Comments[1:]
	}
	return before, after
}

func TestProjectCommentRestorePreservesUnrelatedChanges(t *testing.T) {
	for _, action := range []string{"add", "edit", "delete"} {
		for _, taskEdit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/task_edit=%v", action, taskEdit), func(t *testing.T) {
				ledger, root := newTestLedger(t)
				path := filepath.Join(root, "projects", "alpha", "tasks.md")
				before, after := commentRestoreDocuments(action)
				if taskEdit {
					after.Tasks[0].Title = "Local task edit"
					after.Tasks[0].UpdatedAt = after.Tasks[0].UpdatedAt.Add(time.Minute)
				}
				recordTaskRestore(t, ledger, path, before, after)
				current := after.clone()
				current.ProjectName = "Peer renamed project"
				current.Tasks = append(current.Tasks, mergeTestTask("task-c", "Peer's new task"))
				current.Tasks[1], current.Tasks[2] = current.Tasks[2], current.Tasks[1]
				for step, redo := range []bool{false, true, false, true} {
					// More incoming changes between each restore, including changes
					// to a comment that the original operation left untouched.
					current.Tasks[1].Description = fmt.Sprintf("Peer task edit %d", step)
					for _, comment := range current.Comments {
						if comment.ID == before.Comments[len(before.Comments)-1].ID {
							comment.Text = fmt.Sprintf("Peer comment edit %d", step)
						}
					}
					current.Comments = append(current.Comments, &models.ProjectComment{
						ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", step+3), Text: "Peer addition",
						CreatedAt: before.Tasks[0].CreatedAt.Add(-time.Duration(step+1) * time.Minute),
					})
					if step == 2 {
						current.Tasks = slices.DeleteFunc(current.Tasks, func(task *models.Task) bool { return task.ID == "task-b" })
						current.Comments = slices.DeleteFunc(current.Comments, func(comment *models.ProjectComment) bool {
							return comment.ID == "00000000-0000-4000-8000-000000000002"
						})
					}
					writeFile(t, path, mustWriteCurrentDocument(t, current))
					wanted := current.clone()
					desired := before
					if redo {
						desired = after
					}
					// Construct the expected set without changing any unrelated value.
					targetID := "00000000-0000-4000-8000-000000000001"
					wanted.Comments = nil
					for _, comment := range current.Comments {
						if comment.ID != targetID {
							wanted.Comments = append(wanted.Comments, comment)
						}
					}
					for _, comment := range desired.Comments {
						if comment.ID == targetID {
							wanted.Comments = append(wanted.Comments, comment)
						}
					}
					if taskEdit {
						wanted.Tasks[0] = desired.Tasks[0]
					}
					ledger = NewWorkspaceLedger(ledger.dir, ledger.roots)
					var err error
					if redo {
						_, err = ledger.Redo(GlobalScope())
					} else {
						_, err = ledger.Undo(GlobalScope())
					}
					if err != nil {
						t.Fatalf("step %d: %v", step, err)
					}
					assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, wanted))
					current = wanted.clone()
				}
				if _, err := ledger.Validate(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestProjectCommentRestoreWithoutTasks(t *testing.T) {
	for _, action := range []string{"add", "edit", "delete"} {
		t.Run(action, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := commentRestoreDocuments(action)
			before.Tasks, after.Tasks = nil, nil
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			current.Comments[len(current.Comments)-1].Text = "Peer's untouched comment"
			writeFile(t, path, mustWriteCurrentDocument(t, current))
			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatal(err)
			}
			before.Comments[len(before.Comments)-1].Text = "Peer's untouched comment"
			assertLedgerRestoreContent(t, path, mustWriteCurrentDocument(t, before))
		})
	}
}

func TestProjectCommentRestoreKeepsUnsupportedChangesExact(t *testing.T) {
	for _, change := range []string{"creation_instant", "project_name", "task_addition", "task_deletion", "task_order"} {
		t.Run(change, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			path := filepath.Join(root, "projects", "alpha", "tasks.md")
			before, after := commentRestoreDocuments("edit")
			switch change {
			case "creation_instant":
				after.Comments[0].CreatedAt = after.Comments[0].CreatedAt.Add(time.Hour)
			case "project_name":
				after.ProjectName = "Local rename"
			case "task_addition":
				after.Tasks = append(after.Tasks, mergeTestTask("task-c", "New C"))
			case "task_deletion":
				after.Tasks = after.Tasks[:1]
			case "task_order":
				after.Tasks[0], after.Tasks[1] = after.Tasks[1], after.Tasks[0]
			}
			recordTaskRestore(t, ledger, path, before, after)
			current := after.clone()
			current.ProjectName = "Peer rename"
			content := mustWriteCurrentDocument(t, current)
			writeFile(t, path, content)
			if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
				t.Fatalf("undo error = %v, want ErrStaleArtifact", err)
			}
			assertLedgerRestoreContent(t, path, content)
		})
	}
}

func TestProjectCommentRestoreRefusesChangedTargets(t *testing.T) {
	for _, action := range []string{"add", "edit", "delete"} {
		for _, redo := range []bool{false, true} {
			for _, change := range []string{"text", "timestamp", "presence"} {
				t.Run(fmt.Sprintf("%s/redo=%v/%s", action, redo, change), func(t *testing.T) {
					ledger, root := newTestLedger(t)
					path := filepath.Join(root, "projects", "alpha", "tasks.md")
					before, after := commentRestoreDocuments(action)
					recordTaskRestore(t, ledger, path, before, after)
					current := after.clone()
					writeFile(t, path, mustWriteCurrentDocument(t, current))
					if redo {
						if _, err := ledger.Undo(GlobalScope()); err != nil {
							t.Fatal(err)
						}
						current = before.clone()
					}
					id := "00000000-0000-4000-8000-000000000001"
					comments := indexProjectComments(current.Comments)
					if target := comments[id]; target != nil {
						switch change {
						case "text":
							target.Text = "Peer changed target"
						case "timestamp":
							target.CreatedAt = target.CreatedAt.Add(time.Hour)
						case "presence":
							delete(comments, id)
						}
					} else {
						comments[id] = &models.ProjectComment{ID: id, Text: "Reused identity", CreatedAt: before.Tasks[0].CreatedAt}
					}
					current.Comments = nil
					for _, comment := range comments {
						current.Comments = append(current.Comments, comment)
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
	}
}

func TestProjectCommentRestorePreflightsWholeGroup(t *testing.T) {
	for _, redo := range []bool{false, true} {
		t.Run(fmt.Sprintf("redo=%v", redo), func(t *testing.T) {
			ledger, root := newTestLedger(t)
			first := filepath.Join(root, "projects", "alpha", "tasks.md")
			second := filepath.Join(root, "projects", "beta", "tasks.md")
			before, after := commentRestoreDocuments("edit")
			var artifacts []LedgerArtifact
			for _, path := range []string{first, second} {
				artifacts = append(artifacts, LedgerArtifact{Path: path,
					BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)),
					AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after))})
				writeFile(t, path, mustWriteCurrentDocument(t, after))
			}
			recordLedgerRestoreGroup(t, ledger, "comments", "edit comments", artifacts)
			current := after.clone()
			if redo {
				if _, err := ledger.Undo(GlobalScope()); err != nil {
					t.Fatal(err)
				}
				current = before.clone()
			}
			current.Tasks[1].Title = "Peer task edit"
			firstContent := mustWriteCurrentDocument(t, current)
			writeFile(t, first, firstContent)
			current.Comments[0].Text = "Peer changed second target"
			secondContent := mustWriteCurrentDocument(t, current)
			writeFile(t, second, secondContent)
			var err error
			if redo {
				_, err = ledger.Redo(GlobalScope())
			} else {
				_, err = ledger.Undo(GlobalScope())
			}
			if !errors.Is(err, ErrStaleArtifact) {
				t.Fatalf("restore error = %v, want ErrStaleArtifact", err)
			}
			assertLedgerRestoreContent(t, first, firstContent)
			assertLedgerRestoreContent(t, second, secondContent)
			assertJournalClean(t, root)
		})
	}
}
