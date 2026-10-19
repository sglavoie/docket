package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestProjectAddCommentRefusesAnEmptyBufferWithoutClosing keeps the application
// layer's whitespace rule on the surface that can still do something about it.
func TestProjectAddCommentRefusesAnEmptyBufferWithoutClosing(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "   "},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	prompt := driver.App().operations.TextPrompt
	if prompt == nil {
		t.Fatal("a refused empty project comment closed the popup")
	}
	if prompt.Notice() == "" {
		t.Error("a refused empty project comment said nothing")
	}
	if got := projectCommentsOnDisk(t, workspace); len(got) != 0 {
		t.Fatalf("an empty project comment was stored: %+v", got)
	}
}

// TestProjectAddCommentDiscardsOnlyAfterConfirming is the cancel path: Esc on
// typed text asks first, n gives the text back, and y throws it away without
// writing anything.
func TestProjectAddCommentDiscardsOnlyAfterConfirming(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Half a thought"},
		{Type: UITestActionKey, Key: "esc"},
	})
	if !strings.Contains(driver.Render(), "Discard comment?") {
		t.Fatalf("esc did not ask before discarding:\n%s", driver.Render())
	}

	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "n"}})
	prompt := driver.App().operations.TextPrompt
	if prompt == nil {
		t.Fatal("n closed the popup instead of keeping it")
	}
	if prompt.Value() != "Half a thought" {
		t.Fatalf("value after n = %q, want the text it kept", prompt.Value())
	}

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "esc"},
		{Type: UITestActionKey, Key: "y"},
		{Type: UITestActionWait},
	})
	if driver.App().operations.TextPrompt != nil {
		t.Error("y left the popup open")
	}
	if got := projectCommentsOnDisk(t, workspace); len(got) != 0 {
		t.Fatalf("a discarded project comment was stored: %+v", got)
	}
}

// TestProjectAddCommentSurvivesARefusedSaveAndRetries is the recovery contract.
// A commit can only fail after the popup closed, so the popup comes back on the
// same text, naming the refusal, and the same keystroke commits once the cause
// is gone. A read-only project directory is the honest failure: it stops the
// transaction at the lock, before anything is written.
func TestProjectAddCommentSurvivesARefusedSaveAndRetries(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)

	directory := filepath.Join(workspace.DataDir, "projects", projectCommentFixtureSlug)
	unseal := sealProjectDirectory(t, directory)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Worth keeping through the failure"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	prompt := driver.App().operations.TextPrompt
	if prompt == nil {
		t.Fatalf("a refused save discarded the typed project comment:\n%s", driver.Render())
	}
	if prompt.Value() != "Worth keeping through the failure" {
		t.Fatalf("restored value = %q, want the text that was typed", prompt.Value())
	}
	if prompt.Notice() == "" {
		t.Error("the reopened popup did not say why the save was refused")
	}

	unseal()
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})
	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 1 || stored[0].Text != "Worth keeping through the failure" {
		t.Fatalf("after the retry stored comments = %+v, want the one restored comment", stored)
	}
}

// TestProjectAddCommentKeepsTextWhenTheOwnerDisappears is the half the task
// comment's own reopen path does not satisfy. An owner that vanished or was
// archived while the popup was open must not cost the text and must not fall
// back onto another project: the popup comes back naming the refusal, and
// nothing anywhere is written.
func TestProjectAddCommentKeepsTextWhenTheOwnerDisappears(t *testing.T) {
	for _, tc := range []struct {
		name    string
		vanish  func(app *AppModel, owner *models.Project)
		typed   string
		expects string
	}{
		{
			name: "the project was deleted",
			vanish: func(app *AppModel, owner *models.Project) {
				for id, project := range app.domain.projects {
					if project != nil && project.ID == owner.ID {
						delete(app.domain.projects, id)
					}
				}
			},
			typed:   "Kept through a deletion",
			expects: "no longer available",
		},
		{
			name: "the project was archived",
			vanish: func(_ *AppModel, owner *models.Project) {
				owner.Archived = true
			},
			typed:   "Kept through an archival",
			expects: "archived",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver, workspace, view := addCommentFixtureApp(t)
			seatCursorOnProjectHeader(t, view)

			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "C"},
				{Type: UITestActionText, Text: tc.typed},
			})
			owner := driver.App().projectByID(projectCommentFixtureSlug)
			if owner == nil {
				t.Fatal("the fixture project is not in the shell cache")
			}
			tc.vanish(driver.App(), owner)

			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "ctrl+s"},
				{Type: UITestActionWait},
			})

			prompt := driver.App().operations.TextPrompt
			if prompt == nil {
				t.Fatalf("a vanished owner discarded the typed comment:\n%s", driver.Render())
			}
			if prompt.Value() != tc.typed {
				t.Fatalf("restored value = %q, want %q", prompt.Value(), tc.typed)
			}
			if notice := prompt.Notice(); !strings.Contains(notice, tc.expects) {
				t.Errorf("reopened notice = %q, want it to name %q", notice, tc.expects)
			}
			if got := projectCommentsOnDisk(t, workspace); len(got) != 0 {
				t.Fatalf("a refused owner still stored %+v", got)
			}
		})
	}
}
