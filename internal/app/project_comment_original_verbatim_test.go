package app_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// This file measures one precondition alone: the Original a project-comment
// mutation carries is compared to the stored text byte for byte. Every fixture
// here is written by the canonical schema-17 writer and read back from disk
// before the mutation runs, because the whole question is what the durable
// document holds — a comment body the writer preserves with its surrounding
// whitespace intact must be addressable by exactly those bytes, and a caller
// that read different bytes must be refused whatever the difference is made of.

const (
	verbatimCommentID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	verbatimNeighborID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	verbatimNeighbor   = "the neighboring comment"
)

// paddedCommentBodies are the stored shapes whose padding a trimming comparison
// would erase: ordinary spaces on both ends, surrounding newlines, and Unicode
// whitespace that strings.TrimSpace also removes.
func paddedCommentBodies() map[string]string {
	return map[string]string{
		"leading_and_trailing_spaces": "  note  ",
		"surrounding_newlines":        "\nnote\n",
		"unicode_whitespace":          "\u00a0note\u2003",
	}
}

// verbatimCommentEnv is a real storage-backed project-comment fixture: the
// comments are committed through the production commit service, so the bytes on
// disk are the canonical writer's, and the in-memory collection the mutation
// resolves against is the one read back out of them.
type verbatimCommentEnv struct {
	repo      *countingProjectCommentRepo
	svc       *app.Service
	project   *models.Project
	tasksPath string
	stored    string
	createdAt time.Time
}

// countingProjectCommentRepo counts the transactions the application layer
// opens, so a refusal can be held to reaching storage not at all rather than
// merely to writing nothing.
type countingProjectCommentRepo struct {
	*storageRepo
	commits int
}

func (r *countingProjectCommentRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	r.commits++
	return r.storageRepo.Commit(changesets...)
}

func newVerbatimCommentEnv(t *testing.T, storedText string) *verbatimCommentEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)

	now := projectCommentFixtureTime()
	project := realTestProject(t, dataDir, "alpha", "an unrelated task")
	createdAt := now.Add(-2 * time.Hour)
	project.Comments = []*models.ProjectComment{
		{ID: verbatimCommentID, Text: storedText, CreatedAt: createdAt},
		{ID: verbatimNeighborID, Text: verbatimNeighbor, CreatedAt: now.Add(-time.Hour)},
	}

	repo := &countingProjectCommentRepo{storageRepo: newStorageRepo(commitSvc, project)}
	repo.seed(t)
	repo.commits = 0

	// Reread the seeded document: from here on the fixture is what the canonical
	// writer produced and the live reader parsed, never what the test typed.
	doc, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	project.Comments = doc.Comments
	stored := commentTextByIDOrFail(t, project.Comments, verbatimCommentID)
	if stored != storedText {
		t.Fatalf("stored comment text = %q, want the seeded body %q preserved by the canonical writer", stored, storedText)
	}

	return &verbatimCommentEnv{
		repo:      repo,
		svc:       app.NewServiceWithArtifacts(repo, fixedClock{t: now}, &uuidIDs{}, nil),
		project:   project,
		tasksPath: storage.TasksFilePath(project.Path),
		stored:    stored,
		createdAt: createdAt,
	}
}

func (e *verbatimCommentEnv) target(original string) app.ProjectCommentTarget {
	return app.ProjectCommentTarget{ProjectID: e.project.ID, CommentID: verbatimCommentID, Original: original}
}

// durable reads the project's comments back off disk, which is the only place a
// commit can be observed and the only place a refusal can be proved harmless.
func (e *verbatimCommentEnv) durable(t *testing.T) []*models.ProjectComment {
	t.Helper()
	doc, err := storage.LoadProjectDocument(e.project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	return doc.Comments
}

func (e *verbatimCommentEnv) bytes(t *testing.T) string {
	t.Helper()
	return readFileContent(t, e.tasksPath)
}

// requireUntouched holds a refusal to changing nothing at all: no transaction
// opened, and the document's bytes exactly as they were.
func (e *verbatimCommentEnv) requireUntouched(t *testing.T, before string) {
	t.Helper()
	if e.repo.commits != 0 {
		t.Errorf("commit count = %d, want a refused mutation to open no transaction", e.repo.commits)
	}
	if after := e.bytes(t); after != before {
		t.Errorf("durable bytes changed under a refused mutation:\n before %q\n after  %q", before, after)
	}
}

func (e *verbatimCommentEnv) requireNeighborIntact(t *testing.T, comments []*models.ProjectComment) {
	t.Helper()
	if got := commentTextByIDOrFail(t, comments, verbatimNeighborID); got != verbatimNeighbor {
		t.Errorf("neighboring comment = %q, want %q", got, verbatimNeighbor)
	}
}

func commentByIDOrNil(comments []*models.ProjectComment, id string) *models.ProjectComment {
	for _, comment := range comments {
		if comment != nil && comment.ID == id {
			return comment
		}
	}
	return nil
}

func commentTextByIDOrFail(t *testing.T, comments []*models.ProjectComment, id string) string {
	t.Helper()
	comment := commentByIDOrNil(comments, id)
	if comment == nil {
		t.Fatalf("comment %s is absent from %d stored comments", id, len(comments))
	}
	return comment.Text
}

func TestProjectCommentTargetsCompareOriginalVerbatim(t *testing.T) {
	// An exact Original is honored whatever whitespace the stored body carries:
	// the edit lands, the identity and the creation instant survive it, and the
	// comment beside it is untouched.
	t.Run("edit_padded", func(t *testing.T) {
		for name, stored := range paddedCommentBodies() {
			t.Run(name, func(t *testing.T) {
				env := newVerbatimCommentEnv(t, stored)

				result, err := env.svc.EditProjectComment(env.target(stored), "revised")
				if err != nil {
					t.Fatalf("EditProjectComment against the exact stored text: %v", err)
				}
				if result.CommentID != verbatimCommentID {
					t.Errorf("CommentID = %q, want the addressed comment", result.CommentID)
				}
				if env.repo.commits != 1 {
					t.Errorf("commit count = %d, want exactly one transaction", env.repo.commits)
				}

				comments := env.durable(t)
				edited := commentByIDOrNil(comments, verbatimCommentID)
				if edited == nil {
					t.Fatalf("the edited comment is gone from the durable document")
				}
				if edited.Text != "revised" {
					t.Errorf("durable text = %q, want the replacement", edited.Text)
				}
				if !edited.CreatedAt.Equal(env.createdAt) {
					t.Errorf("CreatedAt = %s, want the immutable instant %s", edited.CreatedAt, env.createdAt)
				}
				env.requireNeighborIntact(t, comments)
			})
		}
	})

	// The same holds for a removal: padding is addressable, not a reason the
	// comment can no longer be deleted.
	t.Run("remove_padded", func(t *testing.T) {
		for name, stored := range paddedCommentBodies() {
			t.Run(name, func(t *testing.T) {
				env := newVerbatimCommentEnv(t, stored)

				result, err := env.svc.RemoveProjectComment(env.target(stored))
				if err != nil {
					t.Fatalf("RemoveProjectComment against the exact stored text: %v", err)
				}
				if result.CommentID != verbatimCommentID {
					t.Errorf("CommentID = %q, want the addressed comment", result.CommentID)
				}

				comments := env.durable(t)
				if commentByIDOrNil(comments, verbatimCommentID) != nil {
					t.Errorf("the addressed comment survived its removal")
				}
				env.requireNeighborIntact(t, comments)
			})
		}
	})

	// A difference made only of whitespace is still a difference: the caller read
	// text the project no longer holds, in either direction, so both mutations
	// refuse and neither reaches storage.
	t.Run("whitespace_changed", func(t *testing.T) {
		for name, pair := range map[string]struct{ stored, stale string }{
			"stored_bare_stale_padded":   {stored: "note", stale: "  note  "},
			"stored_padded_stale_bare":   {stored: "  note  ", stale: "note"},
			"stored_bare_stale_unicode":  {stored: "note", stale: "\u00a0note\u2003"},
			"stored_newlines_stale_bare": {stored: "\nnote\n", stale: "note"},
		} {
			t.Run(name, func(t *testing.T) {
				for _, mutation := range []struct {
					name string
					run  func(*verbatimCommentEnv) error
				}{
					{name: "edit", run: func(env *verbatimCommentEnv) error {
						_, err := env.svc.EditProjectComment(env.target(pair.stale), "replacement")
						return err
					}},
					{name: "remove", run: func(env *verbatimCommentEnv) error {
						_, err := env.svc.RemoveProjectComment(env.target(pair.stale))
						return err
					}},
				} {
					t.Run(mutation.name, func(t *testing.T) {
						env := newVerbatimCommentEnv(t, pair.stored)
						before := env.bytes(t)

						err := mutation.run(env)
						if err == nil {
							t.Fatalf("a %s against stale text %q was accepted for stored %q", mutation.name, pair.stale, pair.stored)
						}
						if !strings.Contains(err.Error(), "changed") {
							t.Errorf("refusal = %v, want it to name the change", err)
						}
						env.requireUntouched(t, before)
						if got := commentTextByIDOrFail(t, env.durable(t), verbatimCommentID); got != pair.stored {
							t.Errorf("durable text = %q, want the stored body %q", got, pair.stored)
						}
					})
				}
			})
		}
	})

	// Presence is still required. A target carrying no readable original names no
	// precondition at all, so it is refused before anything is resolved.
	t.Run("blank_original", func(t *testing.T) {
		for name, original := range map[string]string{
			"empty":             "",
			"spaces":            "   ",
			"tabs_and_newlines": " \n\t ",
			"unicode_only":      "\u00a0\u2003",
		} {
			t.Run(name, func(t *testing.T) {
				env := newVerbatimCommentEnv(t, "  note  ")
				before := env.bytes(t)

				if _, err := env.svc.EditProjectComment(env.target(original), "replacement"); err == nil {
					t.Errorf("an edit with a blank original was accepted")
				}
				if _, err := env.svc.RemoveProjectComment(env.target(original)); err == nil {
					t.Errorf("a delete with a blank original was accepted")
				}
				env.requireUntouched(t, before)
			})
		}
	})

	// Newly supplied replacement text is a separate contract and keeps its own
	// normalization: the precondition is read verbatim, the replacement is
	// trimmed exactly as a task comment's is.
	t.Run("normalized_replacement", func(t *testing.T) {
		env := newVerbatimCommentEnv(t, "  note  ")
		before := env.bytes(t)

		if _, err := env.svc.EditProjectComment(env.target(env.stored), "   \n\t  "); err == nil {
			t.Errorf("a blank replacement was accepted")
		}
		env.requireUntouched(t, before)

		if _, err := env.svc.EditProjectComment(env.target(env.stored), "  revised  "); err != nil {
			t.Fatalf("EditProjectComment: %v", err)
		}
		comments := env.durable(t)
		if got := commentTextByIDOrFail(t, comments, verbatimCommentID); got != "revised" {
			t.Errorf("durable text = %q, want the trimmed replacement", got)
		}
		env.requireNeighborIntact(t, comments)
	})
}
