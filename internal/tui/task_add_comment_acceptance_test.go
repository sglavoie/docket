package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// TestAddCommentAppliesOnTaskRowsAndNowhereElse is the applicability half. `C`
// names one task, so a row that names none — a section, a project header, a
// checklist item whose grammar belongs to the item — must refuse rather than
// reach for a nearby task, and a visual selection must refuse outright because a
// comment is one task's own remark and a selection has nothing to say in one
// voice.
func TestAddCommentAppliesOnTaskRowsAndNowhereElse(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	app := driver.App()
	if _, ok := app.GetOperationRegistry().Descriptor(operation.OperationTaskAddComment); !ok {
		t.Fatal("Add Comment is not registered")
	}
	applicable := func(context operation.CommandContext) bool {
		return app.taskActionOperationApplicable(operation.OperationTaskAddComment, context)
	}

	for _, title := range []string{
		checklistFixturePlainTitle,
		checklistFixtureFoldTitle,
		checklistFixtureParentTitle,
		"Confirm the staging soak",
	} {
		seatCursorOnTaskTitled(t, view, title)
		if !applicable(app.operationCommandContext()) {
			t.Errorf("C refused the task row %q", title)
		}
	}

	seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)
	view.Folds.SetChecklist(view.TaskIdentityForRow(view.Rows[view.Cursor]), true)
	view.RebuildRows()
	item := -1
	for index, row := range view.Rows {
		if row.IsChecklistItem {
			item = index
			break
		}
	}
	if item < 0 {
		t.Fatal("expanding the checklist produced no item row")
	}
	// The checklist row and the two header kinds are refused at execution rather
	// than by applicability, which is the same place `r` is held to that rule: a
	// direct task key stays resolvable over a cursor naming nothing so the
	// executor can give the one canonical missing-target error. What matters here
	// is that the refusal comes before the popup, so no comment is ever typed
	// into a target that was never going to accept it.
	view.Cursor = item
	refusedAtCursor(t, app, "checklist item row")

	seatCursorOnRow(t, view, "section header", func(r outliner.OutlinerRow) bool {
		return r.IsSectionHeader
	})
	refusedAtCursor(t, app, "section header")

	// The project header costs a fold to reach, so the refusal is asserted
	// inside that bracket and the groups are open again on the way out.
	onProjectHeaderCursor(t, view, func() {
		refusedAtCursor(t, app, "project header")
	})

	// A project-qualified section header does name a non-task entity, and that
	// one is refused a step earlier: the key never binds there at all.
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
	header := app.operationCommandContext()
	header.Entity = operation.EntityContext{
		ProjectID: header.Entity.ProjectID, HasSectionHeaderCursor: true,
	}
	header.CurrentEntity = operation.CommandContextCurrentEntity(header.Entity)
	if applicable(header) {
		t.Error("C stayed applicable on a project-qualified section header")
	}

	context := app.operationCommandContext()
	context.Selection.Visual = true
	if applicable(context) {
		t.Error("C applied under a visual selection")
	}
}

// refusedAtCursor executes Add Comment against whatever the cursor names now and
// requires a refusal that never reaches the value step.
func refusedAtCursor(t *testing.T, app *AppModel, name string) {
	t.Helper()
	result := app.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationTaskAddComment,
		Context: app.operationCommandContext(),
	})
	if result.Err == nil {
		t.Errorf("C was accepted on a %s, which names no task", name)
	}
	if result.ValueRequired != nil {
		t.Errorf("a refused %s still raised the comment popup", name)
	}
}

// TestAddCommentStoresWhatWasTyped is the round trip. Newlines, non-ASCII text,
// and the literal sigil characters the capture grammar owns all have to survive
// into the stored comment untouched: a comment is prose, not a capture line, so
// nothing in it is parsed and nothing in it is escaped away.
func TestAddCommentStoresWhatWasTyped(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Root cause: the retry budget was exhausted."},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionText, Text: "Follow-up: ##ops @today #tag ~ref *pin — naïve ✓"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
	if len(task.Comments) != 1 {
		t.Fatalf("comments = %+v, want exactly one", task.Comments)
	}
	want := "Root cause: the retry budget was exhausted.\n" +
		"Follow-up: ##ops @today #tag ~ref *pin — naïve ✓"
	if task.Comments[0].Text != want {
		t.Fatalf("comment = %q, want %q", task.Comments[0].Text, want)
	}
	if driver.App().operations.TextPrompt != nil {
		t.Error("a committed comment left its popup open")
	}

	// Undo takes it back, which is what proves the write went through the same
	// checked transaction and workspace ledger every other task mutation uses
	// rather than a second comment store.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "u"},
		{Type: UITestActionWait},
	})
	if undone := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle); len(undone.Comments) != 0 {
		t.Fatalf("after undo comments = %+v, want none", undone.Comments)
	}
}

// TestAddCommentAcceptsATerminalPaste is the event-path half of "the popup owns
// the keyboard". A bracketed paste is not a run of key presses: the terminal
// hands the whole clipboard over as one tea.PasteMsg, so a popup that read only
// key presses would watch a pasted paragraph fall past it into the Outliner,
// which drops a paste outside quick edit. Every rule typed text obeys holds
// here — newlines and the literal sigil characters survive untouched, the
// comment bound truncates, the discard question still owns the input, and
// pasting is not submitting.
func TestAddCommentAcceptsATerminalPaste(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paste string
		want  string
	}{
		{
			name:  "multiline text is buffered verbatim",
			paste: "Root cause: naïve retry ✓\n@today #tag ~ref *pin $done ##ops",
			want:  "Root cause: naïve retry ✓\n@today #tag ~ref *pin $done ##ops",
		},
		{
			name:  "an oversized paste is cut to the comment bound",
			paste: strings.Repeat("x", taskdetail.CommentCharLimit+5),
			want:  strings.Repeat("x", taskdetail.CommentCharLimit),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver, _, view := addCommentFixtureApp(t)
			seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

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
			if task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle); len(task.Comments) != 0 {
				t.Fatalf("the paste committed a comment by itself: %+v", task.Comments)
			}

			// An armed discard is asking about this very text, so a paste that
			// rewrote it would change the question being answered.
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
			task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
			if len(task.Comments) != 1 || task.Comments[0].Text != tc.want {
				t.Fatalf("after Ctrl+S comments = %+v, want the one pasted comment", task.Comments)
			}
		})
	}
}

// TestAddCommentRefusesAnEmptyBufferWithoutClosing keeps the application layer's
// whitespace rule on the surface that can still do something about it. A
// transaction that would be rejected is never opened, and the popup stays up so
// the answer can be corrected in place.
func TestAddCommentRefusesAnEmptyBufferWithoutClosing(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "   "},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	prompt := driver.App().operations.TextPrompt
	if prompt == nil {
		t.Fatal("a refused empty comment closed the popup")
	}
	if prompt.Notice() == "" {
		t.Error("a refused empty comment said nothing")
	}
	if task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle); len(task.Comments) != 0 {
		t.Fatalf("an empty comment was stored: %+v", task.Comments)
	}
}

// TestAddCommentDiscardsOnlyAfterConfirming is the cancel path. Esc on typed
// text asks first, n gives the text back, and y throws it away without writing
// anything — the same three answers the Comments panel's own add mode gives.
func TestAddCommentDiscardsOnlyAfterConfirming(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

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
	if task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle); len(task.Comments) != 0 {
		t.Fatalf("a discarded comment was stored: %+v", task.Comments)
	}
}

// TestAddCommentSurvivesARefusedSaveAndRetries is the recovery contract. A
// commit can only fail after the popup closed, and a paragraph someone just
// typed is exactly the thing that must not be lost to that: the popup comes back
// on the same text, naming the refusal, and the same keystroke commits once the
// cause is gone. A read-only project directory is the honest failure — it stops
// the transaction at the lock, before anything is written.
func TestAddCommentSurvivesARefusedSaveAndRetries(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

	directory := filepath.Join(workspace.DataDir, "projects", "checklist-delivery")
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
		t.Fatalf("a refused save discarded the typed comment:\n%s", driver.Render())
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

	task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
	if len(task.Comments) != 1 || task.Comments[0].Text != "Worth keeping through the failure" {
		t.Fatalf("after the retry comments = %+v, want the one restored comment", task.Comments)
	}
}

// TestAddCommentTargetsTheTaskItWasOpenedOn is the frozen-identity half. The
// popup names one task, and the value step resolves the target from the frozen
// command context, so the row the cursor happens to be on when the save lands
// cannot redirect the comment.
func TestAddCommentTargetsTheTaskItWasOpenedOn(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Belongs to the row it was opened on"},
	})
	if !strings.Contains(driver.Render(), checklistFixturePlainTitle) {
		t.Errorf("the popup does not name its target task:\n%s", driver.Render())
	}

	// Moving the cursor under an open popup is the redirection this refuses. The
	// projection is written directly because the popup owns the keyboard, which
	// is itself part of why the frozen target is the only one reachable.
	seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	current := driver.App().crossOutliner
	target := projectedTaskTitled(t, current, checklistFixturePlainTitle)
	if len(target.Comments) != 1 {
		t.Fatalf("the frozen target's comments = %+v, want the one that was typed", target.Comments)
	}
	if moved := projectedTaskTitled(t, current, checklistFixtureFoldTitle); len(moved.Comments) != 0 {
		t.Fatalf("the comment followed the cursor onto %q: %+v", checklistFixtureFoldTitle, moved.Comments)
	}
}

// TestAddCommentIsVisibleToEveryReaderOfTheTask closes the loop on what a
// comment is: stored task content, not a side channel. The Details search tier
// finds a word that lives only in the comment, Recent carries the task because
// the append is a touch, and the Comments panel the `;` key opens shows the same
// text the popup wrote.
func TestAddCommentIsVisibleToEveryReaderOfTheTask(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Blocked on the palladium invoice"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	// A word that appears in no title, no description, and no other task, so the
	// only thing that can answer the query is the comment itself.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "/"},
		{Type: UITestActionText, Text: "palladium"},
		{Type: UITestActionWait},
	})
	screen := driver.Render()
	if !strings.Contains(screen, checklistFixturePlainTitle) {
		t.Errorf("a search for the comment's own word did not find its task:\n%s", screen)
	}
	if strings.Contains(screen, checklistFixtureFoldTitle) {
		t.Errorf("the comment search matched a task with no such comment:\n%s", screen)
	}

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "esc"},
		{Type: UITestActionKey, Key: "esc"},
		{Type: UITestActionKey, Key: "g"},
		{Type: UITestActionKey, Key: "r"},
		{Type: UITestActionWait},
	})
	if recent := driver.Render(); !strings.Contains(recent, checklistFixturePlainTitle) {
		t.Errorf("a commented task is not in Recent, so the append wrote no touch time:\n%s", recent)
	}

	// `;` is untouched: it still opens the panel that owns reading, editing,
	// reordering, and deleting, and the comment C wrote is simply there.
	seatCursorOnTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionWait},
	})
	if panel := driver.Render(); !strings.Contains(panel, "Blocked on the palladium invoice") {
		t.Errorf("the Comments panel does not show the comment C wrote:\n%s", panel)
	}
}

// TestAddCommentReachesATerminalTask keeps the key off the lifecycle axis.
// Nothing in the append reads a state, and a finished task is exactly the one
// that still collects the note explaining how it finished.
func TestAddCommentReachesATerminalTask(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "w"},
		{Type: UITestActionKey, Key: "d"},
		{Type: UITestActionWait},
		// Recent admits terminal work, so it is where a completed task still has
		// a row to press a key on.
		{Type: UITestActionKey, Key: "g"},
		{Type: UITestActionKey, Key: "r"},
		{Type: UITestActionWait},
	})

	view = driver.App().crossOutliner
	seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
	if task := projectedTaskTitled(t, view, checklistFixturePlainTitle); !task.IsInState(models.StateDone) {
		t.Fatalf("wd left %q in state %q", task.Title, task.State())
	}

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "C"},
		{Type: UITestActionText, Text: "Closed out after the soak passed"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
	if len(task.Comments) != 1 || task.Comments[0].Text != "Closed out after the soak passed" {
		t.Fatalf("terminal task comments = %+v, want the one appended comment", task.Comments)
	}
}
