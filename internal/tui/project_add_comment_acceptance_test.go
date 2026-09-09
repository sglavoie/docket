package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// The checklist fixture is the right one for `C` on a project: it carries a
// project group header, task rows, checklist item rows, and section headers, and
// it holds exactly one project, so the sole-scope fallback that
// outlinerProjectOperationTargetsHeader narrows has something to fall back onto
// and a leak would be visible rather than latent.
const projectCommentFixtureSlug = "checklist-delivery"

// seatCursorOnProjectHeader parks the cursor on the fixture's project header,
// which is the only row `C` may resolve a project from.
func seatCursorOnProjectHeader(t *testing.T, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnRow(t, view, "project header", func(row outliner.OutlinerRow) bool {
		return row.IsProjectHeader
	})
}

// projectCommentsOnDisk reads the comments the canonical writer actually
// persisted, rather than the ones the shell happens to be holding. A comment
// that never reached tasks.md is not a comment.
func projectCommentsOnDisk(t *testing.T, workspace *uitestWorkspace) []*models.ProjectComment {
	t.Helper()
	path := filepath.Join(workspace.DataDir, "projects", projectCommentFixtureSlug, "tasks.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	document, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return document.Comments
}

// projectCommentsInShell reads the comments the reloaded shell is holding, which
// is what proves the ordinary batch refresh reached this project rather than
// leaving the surface stale until the next launch.
func projectCommentsInShell(t *testing.T, app *AppModel) []*models.ProjectComment {
	t.Helper()
	project := app.projectByID(projectCommentFixtureSlug)
	if project == nil {
		t.Fatalf("the shell no longer holds project %q", projectCommentFixtureSlug)
	}
	return project.Comments
}

// TestProjectAddCommentAppliesOnProjectHeadersAndNowhereElse is the
// disjointness half. `C` is one key over two descriptors, and nothing but their
// applicability keeps them apart, so every row shape has to admit exactly one of
// them — and a section header, a checklist row, an empty projection, and a
// visual selection must admit the project half nowhere, because in a
// single-project workspace the sole-scope fallback would otherwise hand it this
// workspace's only project.
func TestProjectAddCommentAppliesOnProjectHeadersAndNowhereElse(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	app := driver.App()
	if _, ok := app.GetOperationRegistry().Descriptor(operation.OperationProjectAddComment); !ok {
		t.Fatal("Add project comment is not registered")
	}

	cases := []struct {
		name string
		seat func(t *testing.T)
		want operation.OperationID
	}{
		{
			name: "a project header comments on the project",
			seat: func(t *testing.T) { seatCursorOnProjectHeader(t, view) },
			want: operation.OperationProjectAddComment,
		},
		{
			name: "a task row comments on the task",
			seat: func(t *testing.T) { seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle) },
			want: operation.OperationTaskAddComment,
		},
		{
			// A checklist item row names neither a task nor a project, so both
			// halves stand down. What matters here is that the project half is
			// one of them: this workspace holds a single project, and the
			// sole-scope fallback would otherwise hand it that project from a row
			// that never named one.
			name: "a checklist item row comments on nothing",
			seat: func(t *testing.T) {
				seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)
				view.Folds.SetChecklist(view.TaskIdentityForRow(view.Rows[view.Cursor]), true)
				view.RebuildRows()
				seatCursorOnRow(t, view, "checklist item", func(row outliner.OutlinerRow) bool {
					return row.IsChecklistItem
				})
			},
			want: "",
		},
		{
			name: "a section header falls to the task half",
			seat: func(t *testing.T) {
				seatCursorOnRow(t, view, "section header", func(row outliner.OutlinerRow) bool {
					return row.IsSectionHeader && !row.IsProjectHeader
				})
			},
			want: operation.OperationTaskAddComment,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.seat(t)
			assertCommentKeyResolvesTo(t, app, testCase.want)
		})
	}

	// A visual selection has nothing to say in one voice, and it must not reach
	// the project either: a selection of task rows is not a project cursor.
	seatCursorOnProjectHeader(t, view)
	selected := app.operationCommandContext()
	selected.Selection.Visual = true
	if applicableCommentOperations(t, app, selected) != nil {
		t.Error("C applied under a visual selection")
	}
}

// outlinerCommentOperations is `C`'s whole family in Outliner declaration order,
// which is also the order the strategy resolves them in.
var outlinerCommentOperations = []operation.OperationID{
	operation.OperationTaskAddComment,
	operation.OperationProjectAddComment,
}

func applicableCommentOperations(
	t *testing.T, app *AppModel, context operation.CommandContext,
) []operation.OperationID {
	t.Helper()
	var applicable []operation.OperationID
	for _, id := range outlinerCommentOperations {
		descriptor, ok := app.GetOperationRegistry().Descriptor(id)
		if !ok {
			t.Fatalf("%s is not registered", id)
		}
		if descriptor.Applicable(context) {
			applicable = append(applicable, id)
		}
	}
	return applicable
}

// assertCommentKeyResolvesTo asserts which half of `C` a cursor admits. An
// empty want means neither: some rows name no comment owner at all, and the
// project half in particular must never reach past the cursor for one.
func assertCommentKeyResolvesTo(t *testing.T, app *AppModel, want operation.OperationID) {
	t.Helper()
	applicable := applicableCommentOperations(t, app, app.operationCommandContext())
	if want == "" {
		if len(applicable) != 0 {
			t.Fatalf("C applied as %v, want no operation", applicable)
		}
		return
	}
	if len(applicable) != 1 {
		t.Fatalf("C applied as %v, want exactly one operation", applicable)
	}
	if applicable[0] != want {
		t.Fatalf("C resolved to %s, want %s", applicable[0], want)
	}
}

// TestProjectAddCommentOnAnEmptyProjectionCommentsOnNothing is the sharpest
// version of the narrowing: no row means no cursor, and the workspace's only
// project is still one keystroke from a comment if anything in the family
// reaches past the cursor for a target.
func TestProjectAddCommentOnAnEmptyProjectionCommentsOnNothing(t *testing.T) {
	driver, workspace, _ := addCommentFixtureApp(t)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "/"},
		{Type: UITestActionText, Text: "zzzzz no task carries this"},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	for _, row := range driver.App().crossOutliner.Rows {
		if row.IsTaskRow() {
			t.Fatalf("the search still matched %q, so this is not the empty case", row.Task.Title)
		}
	}
	assertCommentKeyResolvesTo(t, driver.App(), operation.OperationTaskAddComment)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionWait},
	})
	if driver.App().operations.TextPrompt != nil {
		t.Fatalf("C raised a comment popup over an empty projection:\n%s", driver.Render())
	}
	if got := projectCommentsOnDisk(t, workspace); len(got) != 0 {
		t.Fatalf("an empty projection wrote %d project comments", len(got))
	}
}

// TestProjectAddCommentStoresWhatWasTyped is the round trip, and the one that
// keeps the two halves of `C` apart at the storage layer too: the project gains
// exactly one comment and no task gains any.
func TestProjectAddCommentStoresWhatWasTyped(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Scope note: the soak owns the window."},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionText, Text: "Follow-up: ##ops @today #tag ~ref *pin — naïve ✓"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	want := "Scope note: the soak owns the window.\n" +
		"Follow-up: ##ops @today #tag ~ref *pin — naïve ✓"
	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 1 {
		t.Fatalf("stored project comments = %+v, want exactly one", stored)
	}
	if stored[0].Text != want {
		t.Fatalf("stored text = %q, want %q", stored[0].Text, want)
	}
	if stored[0].ID == "" || stored[0].CreatedAt.IsZero() {
		t.Errorf("stored comment carries no identity or instant: %+v", stored[0])
	}
	if driver.App().operations.TextPrompt != nil {
		t.Error("a committed project comment left its popup open")
	}

	// The ordinary batch refresh is what makes the write visible without a
	// relaunch, so the shell has to be holding it too.
	if held := projectCommentsInShell(t, driver.App()); len(held) != 1 {
		t.Fatalf("the reloaded shell holds %d project comments, want the one just written", len(held))
	}

	// No task gained a comment: the two halves of C write different records.
	for _, row := range driver.App().crossOutliner.Rows {
		if row.IsTaskRow() && len(row.Task.Comments) != 0 {
			t.Errorf("task %q gained comments from a project comment: %+v", row.Task.Title, row.Task.Comments)
		}
	}

	// Undo takes it back and redo puts it again, which is what proves the write
	// went through the same checked transaction and workspace ledger as every
	// other mutation rather than a second comment store.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "u"},
		{Type: UITestActionWait},
	})
	if undone := projectCommentsOnDisk(t, workspace); len(undone) != 0 {
		t.Fatalf("after undo stored comments = %+v, want none", undone)
	}
	if held := projectCommentsInShell(t, driver.App()); len(held) != 0 {
		t.Fatalf("after undo the shell still holds %+v", held)
	}

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+r"},
		{Type: UITestActionWait},
	})
	redone := projectCommentsOnDisk(t, workspace)
	if len(redone) != 1 || redone[0].Text != want {
		t.Fatalf("after redo stored comments = %+v, want the one that was typed", redone)
	}
}

// TestProjectAddCommentAcceptsATerminalPaste is the event-path half. A
// bracketed paste is one tea.PasteMsg rather than a run of key presses, and the
// project prompt shares the task prompt's whole contract: the text is buffered
// verbatim, an oversized paste is cut to the same bound, an armed discard
// swallows the paste rather than rewriting the text it is asking about, and
// pasting is not submitting.
func TestProjectAddCommentAcceptsATerminalPaste(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paste string
		want  string
	}{
		{
			name:  "multiline text is buffered verbatim",
			paste: "Scope: naïve retry ✓\n@today #tag ~ref *pin $done ##ops",
			want:  "Scope: naïve retry ✓\n@today #tag ~ref *pin $done ##ops",
		},
		{
			name:  "an oversized paste is cut to the comment bound",
			paste: strings.Repeat("x", taskdetail.CommentCharLimit+5),
			want:  strings.Repeat("x", taskdetail.CommentCharLimit),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver, workspace, view := addCommentFixtureApp(t)
			seatCursorOnProjectHeader(t, view)

			runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "C"}})
			driver.dispatch(tea.PasteMsg{Content: tc.paste})
			driver.drain()

			prompt := driver.App().operations.TextPrompt
			if prompt == nil {
				t.Fatalf("the paste closed the popup:\n%s", driver.Render())
			}
			if prompt.Value() != tc.want {
				t.Fatalf("pasted buffer = %q, want %q", prompt.Value(), tc.want)
			}
			if got := projectCommentsOnDisk(t, workspace); len(got) != 0 {
				t.Fatalf("the paste committed a comment by itself: %+v", got)
			}

			runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "esc"}})
			if !prompt.ConfirmingDiscard() {
				t.Fatalf("esc did not ask before discarding the pasted text:\n%s", driver.Render())
			}
			driver.dispatch(tea.PasteMsg{Content: "never entered"})
			driver.drain()
			if prompt.Value() != tc.want {
				t.Fatalf("a paste reached the buffer under the discard question: %q", prompt.Value())
			}

			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "n"},
				{Type: UITestActionKey, Key: "ctrl+s"},
				{Type: UITestActionWait},
			})
			stored := projectCommentsOnDisk(t, workspace)
			if len(stored) != 1 || stored[0].Text != tc.want {
				t.Fatalf("after Ctrl+S stored comments = %+v, want the one pasted comment", stored)
			}
		})
	}
}

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
	if err := os.Chmod(directory, 0o555); err != nil {
		t.Fatalf("seal the project directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o755) })

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

	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatalf("unseal the project directory: %v", err)
	}
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})
	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 1 || stored[0].Text != "Worth keeping through the failure" {
		t.Fatalf("after the retry stored comments = %+v, want the one restored comment", stored)
	}
}

// TestProjectAddCommentSubmitsOnce keeps a second submit key from opening a
// second transaction. The prompt emits one selection per open, so a doubled
// Ctrl+S — a repeat, a stuck key, an impatient press — cannot duplicate a
// commit.
func TestProjectAddCommentSubmitsOnce(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Exactly once"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 1 {
		t.Fatalf("three submit keys stored %+v, want exactly one comment", stored)
	}
}

// TestProjectAddCommentTargetsTheProjectItWasOpenedOn is the frozen-owner half.
// The value step resolves its target from the frozen command context, so moving
// the cursor onto a task row under an open popup cannot redirect the text onto
// that task, and renaming the project cannot redirect it either: the freeze is
// on identity, not on the name the header happened to show.
func TestProjectAddCommentTargetsTheProjectItWasOpenedOn(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Belongs to the project it was opened on"},
	})
	if !strings.Contains(driver.Render(), checklistFixtureProjectName) {
		t.Errorf("the popup does not name its target project:\n%s", driver.Render())
	}

	// Moving the cursor and renaming the owner are the two redirections this
	// refuses. Both are written straight onto the projection and the shell cache
	// because the popup owns the keyboard, which is itself part of why the frozen
	// target is the only one reachable.
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
	owner := driver.App().projectByID(projectCommentFixtureSlug)
	if owner == nil {
		t.Fatal("the fixture project is not in the shell cache")
	}
	owner.Name = "Renamed Mid Prompt"

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 1 || stored[0].Text != "Belongs to the project it was opened on" {
		t.Fatalf("the frozen owner's comments = %+v, want the one that was typed", stored)
	}
	if task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle); len(task.Comments) != 0 {
		t.Fatalf("the comment followed the cursor onto a task: %+v", task.Comments)
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

// TestProjectAddCommentPreservesPeek keeps the success path off the Peek axis.
// The commit rides the ordinary batch result, which reloads the touched project
// and settles the cursor without touching Peek, so an open detail pane is still
// open afterwards and a closed one is still closed.
func TestProjectAddCommentPreservesPeek(t *testing.T) {
	for _, open := range []bool{true, false} {
		name := "peek closed stays closed"
		if open {
			name = "peek open stays open"
		}
		t.Run(name, func(t *testing.T) {
			driver, workspace, view := addCommentFixtureApp(t)
			if open {
				seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
				runChecklistActions(t, driver, []UITestAction{
					{Type: UITestActionKey, Key: "b"},
					{Type: UITestActionWait},
				})
				if !driver.App().crossOutliner.PeekMode {
					t.Fatalf("b did not open Peek:\n%s", driver.Render())
				}
			}

			seatCursorOnProjectHeader(t, driver.App().crossOutliner)
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "C"},
				{Type: UITestActionText, Text: "Peek is not this key's business"},
				{Type: UITestActionKey, Key: "ctrl+s"},
				{Type: UITestActionWait},
			})

			if got := projectCommentsOnDisk(t, workspace); len(got) != 1 {
				t.Fatalf("stored comments = %+v, want the one that was typed", got)
			}
			if got := driver.App().crossOutliner.PeekMode; got != open {
				t.Errorf("PeekMode after the commit = %v, want the unchanged %v", got, open)
			}
		})
	}
}
