package taskdetail

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// The identities below are the two comments this file's cases are about: the one
// the panel adds and the one another writer lands while that add is in flight.
// Both are scripted rather than minted so every assertion can name the comment it
// means without reading the list back to find out.
const (
	locallyAddedCommentID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	concurrentCommentID   = "12345678-90ab-cdef-1234-567890abcdef"
)

// localAddText is the body the panel submits. The equal-text case gives the
// concurrent comment the same one, which is what makes text useless as an
// address there.
const localAddText = "Added right here"

// localAddInstant is what the scripted clock stamps on the added comment. It is
// later than both fixture comments, so the concurrent addition can be placed on
// either side of it without disturbing the rest of the list.
var localAddInstant = time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

// TestProjectCommentAddSelectionUsesCommittedID holds the address a confirmed
// add lands on: the identity the application layer committed, and nothing else.
//
// The defect it locks is invisible with one writer. A project's comments are
// re-projected newest first after every commit, so a comment that arrived from
// another surface, another machine, or an undo while the local add was pending is
// new to this panel too — and a cursor placed on "the row that was not here
// before" then lands on that other writer's comment, aiming the next read, copy,
// edit, and delete at it. The committed identity is the only thing that tells the
// two additions apart, which is why it is carried from the use case through the
// coordinator to the panel rather than inferred at the end of it.
func TestProjectCommentAddSelectionUsesCommittedID(t *testing.T) {
	cases := []struct {
		name string
		// text and at describe the concurrent comment; topmost names the identity
		// the case arranges to be drawn at row 0, which is what says the case is
		// still testing what it was written to test.
		text    string
		at      time.Time
		topmost string
	}{
		{
			name:    "concurrent_newer",
			text:    "Landed from another surface",
			at:      localAddInstant.Add(time.Hour),
			topmost: concurrentCommentID,
		},
		{
			name:    "concurrent_equal_text",
			text:    localAddText,
			at:      localAddInstant.Add(time.Hour),
			topmost: concurrentCommentID,
		},
		{
			name:    "concurrent_older",
			text:    "Landed from another surface",
			at:      localAddInstant.Add(-time.Hour),
			topmost: locallyAddedCommentID,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			project := projectCommentsFixture(t)
			model := newConcurrentAddProjectCommentsPanel(t, project, &models.ProjectComment{
				ID: concurrentCommentID, Text: test.text, CreatedAt: test.at,
			})
			model.Cursor = 1 // the older comment, at the bottom
			model.AddMode = true
			model.TextInput.SetValue(localAddText)

			cmd := model.saveComment()
			if cmd == nil {
				t.Fatal("the add produced no commit command")
			}
			outcome := cmd()

			// The submission window: the transaction has committed and the other
			// writer's comment is already in the workspace, but the panel has not
			// applied the outcome yet, so nothing on screen may have moved.
			if !model.AddMode || model.TextInput.Value() != localAddText {
				t.Fatalf("the pending add lost its composer: adding=%v buffer=%q",
					model.AddMode, model.TextInput.Value())
			}
			if entries := model.Entries(); len(entries) != 2 || model.Cursor != 1 ||
				entries[model.Cursor].ID != olderCommentID {
				t.Fatalf("a pending add moved the selection: cursor=%d over %+v", model.Cursor, entries)
			}

			added, ok := outcome.(CommentAddedMsg)
			if !ok {
				t.Fatalf("the add commit resolved to %T, want CommentAddedMsg", outcome)
			}
			if added.CommentID != locallyAddedCommentID {
				t.Fatalf("the confirmed add carried comment id %q, want the committed identity %q",
					added.CommentID, locallyAddedCommentID)
			}
			model.Update(added)

			// Both additions are independent, so storage keeps both: a selection
			// defect must never be reported as a merge that dropped one.
			assertCommentIdentitiesStored(t, project,
				olderCommentID, newerCommentID, locallyAddedCommentID, concurrentCommentID)

			entries := model.Entries()
			if len(entries) != 4 {
				t.Fatalf("panel entries = %+v, want all four comments", entries)
			}
			if entries[0].ID != test.topmost {
				t.Fatalf("row 0 holds %q, want %q: this case no longer arranges the order it names",
					entries[0].ID, test.topmost)
			}
			if model.Cursor < 0 || model.Cursor >= len(entries) {
				t.Fatalf("cursor = %d over %d entries", model.Cursor, len(entries))
			}
			if selected := entries[model.Cursor].ID; selected != added.CommentID {
				t.Fatalf("the add selected %q, want the comment it committed, %q",
					selected, added.CommentID)
			}
		})
	}

	// A task comment carries no identity anywhere: the use case addresses it by
	// position, the owner projects it in stored order, and an add appends. The
	// committed identity is empty for it, and the cursor still follows the append.
	t.Run("task_owner_compatibility", func(t *testing.T) {
		project, task := commentsFixture(t, "First", "Second")
		model := newTestCommentsPanel(task, project)
		model.Cursor = 0
		model.AddMode = true
		model.TextInput.SetValue("Appended here")

		cmd := model.saveComment()
		if cmd == nil {
			t.Fatal("the add produced no commit command")
		}
		outcome := cmd()
		if model.Cursor != 0 || len(model.Entries()) != 2 {
			t.Fatalf("a pending add moved the selection: cursor=%d over %+v", model.Cursor, model.Entries())
		}

		added, ok := outcome.(CommentAddedMsg)
		if !ok {
			t.Fatalf("the add commit resolved to %T, want CommentAddedMsg", outcome)
		}
		if added.CommentID != "" {
			t.Fatalf("a task comment reported the identity %q it does not store", added.CommentID)
		}
		model.Update(added)

		entries := model.Entries()
		if len(entries) != 3 {
			t.Fatalf("panel entries = %+v, want the three task comments", entries)
		}
		if model.Cursor != 2 || entries[model.Cursor].Text != "Appended here" {
			t.Fatalf("the add selected row %d (%+v), want the appended comment at the bottom",
				model.Cursor, entries)
		}
	})
}

// newConcurrentAddProjectCommentsPanel mounts the production project adapter over
// the production coordinator, with the use cases reaching a repository that lands
// one other writer's comment the instant the local add commits.
//
// The identity and the instant of the local add are scripted, so the assertions
// name the committed comment from outside the message under test rather than
// believing whatever the panel reports.
func newConcurrentAddProjectCommentsPanel(
	t *testing.T,
	project *models.Project,
	remote *models.ProjectComment,
) *CommentsModel {
	t.Helper()
	repo := &concurrentCommentRepo{testAppRepo: newTestAppRepo(project), project: project, remote: remote}
	service := app.NewServiceWithArtifacts(
		repo,
		fixedCommentClock{at: localAddInstant},
		&scriptedCommentIDs{t: t, ids: []string{locallyAddedCommentID}},
		nil,
	)
	comments := coordinator.NewSecondaryCoordinators(service).ProjectComment
	return newProjectCommentsPanel(project, comments, func(id string) *models.Project {
		if project.ID == id {
			return project
		}
		return nil
	})
}

// concurrentCommentRepo is the other writer. It commits the panel's own
// transaction through the ordinary test repository and then lands a second
// comment on the same project, in the workspace and in the in-memory catalog the
// shell's reload would have refreshed — which puts that addition squarely inside
// the window between the submission and the panel's completion reload.
//
// It fires once. The panel reloads after the commit, and a repository that added
// a comment on every write would make the list grow forever instead of describing
// one race.
type concurrentCommentRepo struct {
	*testAppRepo
	project *models.Project
	remote  *models.ProjectComment
	landed  bool
}

func (r *concurrentCommentRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	results, err := r.testAppRepo.Commit(changesets...)
	if err != nil || r.landed || r.remote == nil {
		return results, err
	}
	r.landed = true
	r.project.Comments = append(r.project.Comments, r.remote)
	models.SortProjectComments(r.project.Comments)
	if saveErr := storage.SaveProject(r.project); saveErr != nil {
		return nil, saveErr
	}
	return results, nil
}

// fixedCommentClock stamps every minted comment with one known instant.
type fixedCommentClock struct{ at time.Time }

func (c fixedCommentClock) Now() time.Time { return c.at }

// scriptedCommentIDs mints the identities this file names, in order, and fails
// the test rather than inventing one: an unscripted identity would make the
// assertions about "the committed comment" unfalsifiable.
type scriptedCommentIDs struct {
	t    *testing.T
	ids  []string
	next int
}

func (g *scriptedCommentIDs) NewID() string {
	if g.next >= len(g.ids) {
		g.t.Fatalf("the use cases minted %d identities, but only %d are scripted", g.next+1, len(g.ids))
		return ""
	}
	id := g.ids[g.next]
	g.next++
	return id
}

// assertCommentIdentitiesStored names the identities the durable document holds,
// each exactly once. It reads the file rather than the in-memory project, because
// what survived a two-writer window is a question about what was written.
func assertCommentIdentitiesStored(t *testing.T, project *models.Project, want ...string) {
	t.Helper()
	reloaded, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if err = storage.LoadProject(reloaded); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	counts := make(map[string]int, len(reloaded.Comments))
	for _, comment := range reloaded.Comments {
		if comment == nil {
			t.Fatalf("the stored document holds a nil comment: %+v", reloaded.Comments)
		}
		counts[comment.ID]++
	}
	for _, id := range want {
		if counts[id] != 1 {
			t.Fatalf("stored comment %s appears %d time(s), want exactly one: %+v",
				id, counts[id], reloaded.Comments)
		}
	}
	if len(counts) != len(want) {
		t.Fatalf("stored comments = %+v, want exactly %d identities", reloaded.Comments, len(want))
	}
}
