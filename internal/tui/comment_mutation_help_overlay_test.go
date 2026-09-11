package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// This file holds open the one window the product otherwise closes too fast to
// see: the interval between a comment mutation being submitted and its outcome
// coming back. `?` fits inside that interval, and Help is an overlay, so the
// panel that submitted the change is still mounted underneath it — still holding
// the pending submission, and still the only model that can settle it.
//
// Handing that outcome to whatever presentation happens to be on top instead
// left the panel pending forever, because nothing else clears it: a refresh
// reloads entries and clamps the cursor and deliberately stops there. A panel
// with a pending submission refuses every later add, edit, and delete, so the
// user's only recovery was to close Comments and open it again.
//
// Every case below drives the real shell over a real workspace and holds a real
// outcome. The transaction runs; only its message is caught before the shell
// sees it. Then Help goes up, and only then is the message delivered through
// AppModel.Update.

const (
	overlayKeptComment      = "First remark"
	overlayDoomedComment    = "Second remark"
	overlayAddedComment     = "Added after the overlay"
	overlayReplacementDraft = "Typed into the replacement panel"
)

// commentOverlayOwner is one of the two entities the shared Comments panel
// commits for. They are covered together because the defect is the shell's
// dispatch rather than either adapter's: the outcome is correlated by owner key
// and token no matter which owner minted it, and both correlations were being
// delivered to Help.
type commentOverlayOwner struct {
	// name is the subtest prefix and the owner kind in every failure line.
	name string
	// open seats the Outliner cursor on the entity and mounts the panel over it.
	open func(t *testing.T, driver *UITestDriver, press func(string))
	// stored reads the comments the canonical writer persisted for that entity,
	// in canonical storage order. What never reached tasks.md is not a comment.
	stored func(t *testing.T, workspace *uitestWorkspace) []string
	// breakTheTarget makes the frozen delete uncommittable, from outside this
	// shell. A refusal the panel can see coming is refused before submission and
	// never opens the window this file is about, so the perturbation has to be one
	// only the application layer's fresh snapshot can notice.
	breakTheTarget func(t *testing.T, driver *UITestDriver, workspace *uitestWorkspace)
	// refusal is a fragment of the application layer's own sentence.
	refusal string
}

// projectCommentOverlayOwner is the identity-addressed owner. Its mutations
// carry the comment's id and the text the panel last read, so rewriting that
// text from another surface is exactly what the precondition exists to catch.
func projectCommentOverlayOwner() commentOverlayOwner {
	return commentOverlayOwner{
		name: "project",
		open: func(t *testing.T, driver *UITestDriver, press func(string)) {
			t.Helper()
			seatCursorOnProjectHeader(t, driver.App().crossOutliner)
			press(";")
		},
		stored: func(t *testing.T, workspace *uitestWorkspace) []string {
			t.Helper()
			return projectCommentTextsOnDisk(t, workspace, projectCommentFixtureSlug)
		},
		breakTheTarget: func(t *testing.T, driver *UITestDriver, workspace *uitestWorkspace) {
			t.Helper()
			owner := driver.App().projectByID(projectCommentFixtureSlug)
			if owner == nil {
				t.Fatal("the fixture project is not in the shell cache")
			}
			stored := projectCommentsOnDisk(t, workspace)
			if len(stored) == 0 {
				t.Fatal("the fixture project carries no comment to rewrite")
			}
			doomed := stored[len(stored)-1]
			target := app.ProjectCommentTarget{
				ProjectID: owner.ID,
				CommentID: doomed.ID,
				Original:  doomed.Text,
			}
			if _, err := externalProjectCommentWriter(t, workspace).
				EditProjectComment(target, "Rewritten by another surface"); err != nil {
				t.Fatalf("the external writer could not rewrite the target: %v", err)
			}
		},
		refusal: "comment changed since it was read",
	}
}

// taskCommentOverlayOwner is the positional one. A task comment carries no
// identity, so the only precondition the application layer can enforce is the
// index, and removing the comment above the frozen one is what puts it out of
// range.
func taskCommentOverlayOwner() commentOverlayOwner {
	return commentOverlayOwner{
		name: "task",
		open: func(t *testing.T, driver *UITestDriver, press func(string)) {
			t.Helper()
			seatCursorOnTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
			press(";")
		},
		stored: func(t *testing.T, workspace *uitestWorkspace) []string {
			t.Helper()
			return taskCommentTextsOnDisk(t, workspace, checklistFixturePlainTitle)
		},
		breakTheTarget: func(t *testing.T, driver *UITestDriver, workspace *uitestWorkspace) {
			t.Helper()
			project := driver.App().projectByID(projectCommentFixtureSlug)
			if project == nil {
				t.Fatal("the fixture project is not in the shell cache")
			}
			task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
			locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
			if _, err := externalProjectCommentWriter(t, workspace).RemoveComment(locator, 0); err != nil {
				t.Fatalf("the external writer could not remove the neighbor: %v", err)
			}
		},
		refusal: "comment index 1 out of range",
	}
}

// TestCommentMutationCompletionSurvivesHelpOverlay is the whole contract for a
// comment outcome that arrives under Help: it reaches the panel that submitted
// it, it settles that panel once, Help stays where the user put it, and a panel
// that is gone or replaced still drops what is no longer its own.
func TestCommentMutationCompletionSurvivesHelpOverlay(t *testing.T) {
	for _, owner := range []commentOverlayOwner{projectCommentOverlayOwner(), taskCommentOverlayOwner()} {
		t.Run(owner.name+"_success", func(t *testing.T) {
			runCommentOverlayCompletion(t, owner, false)
		})
		t.Run(owner.name+"_failure", func(t *testing.T) {
			runCommentOverlayCompletion(t, owner, true)
		})
	}

	// The panel the user walked away from is not a panel to settle. The commit it
	// started still lands — it was already in flight — but its outcome must not
	// put Comments back on the route behind the user's Esc.
	t.Run("abandoned_panel", func(t *testing.T) {
		driver, workspace, _ := addCommentFixtureApp(t)
		press, typeText := commentOverlayKeys(t, driver)
		owner := taskCommentOverlayOwner()

		owner.open(t, driver, press)
		press("a")
		typeText(overlayDoomedComment)
		press("ctrl+s")
		driver.Wait(defaultWaitTimeout)

		abandoned := commentsPanelUnderTheRoute(t, driver)
		confirmDeleteAtCursor(t, driver, press, abandoned)
		held := holdCommentOutcome(t, driver)

		// Help goes up, comes back down, and then the user leaves entirely.
		press("?")
		press("?")
		press("esc")
		if got := driver.App().currentView(); got != presentation.ViewCrossProjectOutliner {
			t.Fatalf("Esc left the route on %s, want the Outliner the panel was opened from", got)
		}

		deliverHeldOutcome(driver, held)

		if got := driver.App().currentView(); got != presentation.ViewCrossProjectOutliner {
			t.Fatalf("the abandoned outcome moved the route to %s", got)
		}
		if names := driver.viewStackNames(); slices.Contains(names, presentation.ViewTaskComments.String()) {
			t.Fatalf("the abandoned outcome put Comments back on the route: %v", names)
		}
		if driver.App().entities.ActiveCommentsPanel(driver.App()) != nil {
			t.Fatal("a Comments panel is active again after an outcome the user walked away from")
		}
		if abandoned.Err != nil {
			t.Fatalf("the abandoned panel adopted the outcome's error %v", abandoned.Err)
		}

		// What was dropped is the delivery, never the commit.
		if got := owner.stored(t, workspace); len(got) != 0 {
			t.Fatalf("stored comments = %v, want the confirmed delete to have removed the only one", got)
		}

		// And nothing about the abandonment poisons the next panel over the same
		// entity: it opens clean and commits.
		owner.open(t, driver, press)
		press("a")
		typeText(overlayAddedComment)
		press("ctrl+s")
		driver.Wait(defaultWaitTimeout)
		if got := owner.stored(t, workspace); !reflect.DeepEqual(got, []string{overlayAddedComment}) {
			t.Fatalf("the reopened panel stored %v, want the one comment it was given", got)
		}
	})

	// A replacement panel is the harder half: it is mounted under the same
	// presentation the stale outcome names, so only the owner key and the token
	// can tell it apart from its predecessor. It has a submission, a draft, and a
	// selection of its own, and the stale outcome owes all three.
	t.Run("replaced_owner", func(t *testing.T) {
		driver, workspace, _ := addCommentFixtureApp(t)
		press, typeText := commentOverlayKeys(t, driver)
		taskOwner := taskCommentOverlayOwner()
		projectOwner := projectCommentOverlayOwner()

		taskOwner.open(t, driver, press)
		press("a")
		typeText(overlayDoomedComment)
		press("ctrl+s")
		driver.Wait(defaultWaitTimeout)

		confirmDeleteAtCursor(t, driver, press, commentsPanelUnderTheRoute(t, driver))
		held := holdCommentOutcome(t, driver)

		press("?")
		press("?")
		press("esc")

		// Same presentation, different owner.
		projectOwner.open(t, driver, press)
		replacement := commentsPanelUnderTheRoute(t, driver)
		seeded := []string{"Older project remark", "Newer project remark"}
		for _, text := range seeded {
			press("a")
			typeText(text)
			press("ctrl+s")
			driver.Wait(defaultWaitTimeout)
		}
		press("j")
		if replacement.Cursor != 1 {
			t.Fatalf("cursor = %d, want the second row so the selection is not the default", replacement.Cursor)
		}

		// Its own submission stays outstanding for the whole of the stale delivery.
		press("a")
		typeText(overlayReplacementDraft)
		if err := driver.SendKey("ctrl+s"); err != nil {
			t.Fatalf("SendKey(ctrl+s): %v", err)
		}
		outstanding := driver.pendingCmd
		driver.pendingCmd = nil
		if outstanding == nil {
			t.Fatal("the replacement's own add raised no commit to leave outstanding")
		}
		draft := replacement.TextInput.Value()
		cursor := replacement.Cursor
		entries := panelCommentTexts(replacement)

		deliverHeldOutcome(driver, held)

		if got := driver.App().currentView(); got != presentation.ViewTaskComments {
			t.Fatalf("the stale outcome moved the route to %s", got)
		}
		if commentsPanelUnderTheRoute(t, driver) != replacement {
			t.Fatal("the stale outcome replaced the panel that is on the route")
		}
		if !replacement.AddMode {
			t.Fatalf("the stale outcome closed the replacement's composer:\n%s", driver.Render())
		}
		if got := replacement.TextInput.Value(); got != draft {
			t.Fatalf("the replacement's draft = %q, want the untouched %q", got, draft)
		}
		if replacement.Cursor != cursor {
			t.Fatalf("the replacement's cursor = %d, want the untouched %d", replacement.Cursor, cursor)
		}
		if got := panelCommentTexts(replacement); !reflect.DeepEqual(got, entries) {
			t.Fatalf("the replacement lists %v, want the untouched %v", got, entries)
		}

		// Its pending state is untouched too, which only the duplicate gate can say.
		press("ctrl+s")
		if got := replacement.StatusMessage; got != "Comment change already in progress" {
			t.Fatalf("a second submission left status %q, want the duplicate refusal", got)
		}

		// And when the replacement's own outcome finally arrives, it settles on its
		// own correlation.
		driver.processCmds([]tea.Cmd{outstanding}, driver.drainOpts())
		driver.Wait(defaultWaitTimeout)
		if replacement.AddMode {
			t.Fatalf("the replacement's own outcome left its composer open:\n%s", driver.Render())
		}
		want := append(slices.Clone(seeded), overlayReplacementDraft)
		if got := projectOwner.stored(t, workspace); !reflect.DeepEqual(got, want) {
			t.Fatalf("stored project comments = %v, want %v", got, want)
		}
	})
}

// runCommentOverlayCompletion is the shared walk for both owners and both
// outcomes: seed two comments, freeze a delete on the one under the cursor,
// optionally break it from outside, confirm it, hold what it answers, raise
// Help, and deliver the answer to a panel nobody can see.
func runCommentOverlayCompletion(t *testing.T, owner commentOverlayOwner, refused bool) {
	t.Helper()
	driver, workspace, _ := addCommentFixtureApp(t)
	press, typeText := commentOverlayKeys(t, driver)

	owner.open(t, driver, press)
	panel := commentsPanelUnderTheRoute(t, driver)
	for _, text := range []string{overlayKeptComment, overlayDoomedComment} {
		press("a")
		typeText(text)
		press("ctrl+s")
		driver.Wait(defaultWaitTimeout)
	}
	// Both owners land the cursor on the comment just added, whichever end of
	// their own order that is, so the frozen target is the same comment for both.
	if got := panelCommentTexts(panel); len(got) != 2 || got[panel.Cursor] != overlayDoomedComment {
		t.Fatalf("the panel lists %v with the cursor at %d, want it on %q",
			got, panel.Cursor, overlayDoomedComment)
	}

	confirmDeleteAtCursor(t, driver, press, panel)
	if refused {
		owner.breakTheTarget(t, driver, workspace)
	}
	// Read the workspace after the perturbation and before the commit: this is
	// what a refusal has to leave exactly as it found it.
	stored := owner.stored(t, workspace)

	held := holdCommentOutcome(t, driver)

	press("?")
	if got := driver.App().currentView(); got != presentation.ViewHelp {
		t.Fatalf("? left the route on %s, want Help covering the panel", got)
	}
	if commentsPanelUnderTheRoute(t, driver) != panel {
		t.Fatal("Help replaced the Comments model instead of covering it")
	}

	deliverHeldOutcome(driver, held)

	if got := driver.App().currentView(); got != presentation.ViewHelp {
		t.Fatalf("the outcome moved the route to %s, want Help still visible", got)
	}

	after := owner.stored(t, workspace)
	if refused {
		if !reflect.DeepEqual(after, stored) {
			t.Fatalf("the refused delete left stored comments %v, want the untouched %v", after, stored)
		}
		if panel.Err == nil || !strings.Contains(panel.Err.Error(), owner.refusal) {
			t.Fatalf("the panel holds %v, want the refusal naming %q", panel.Err, owner.refusal)
		}
	} else {
		want := slices.DeleteFunc(slices.Clone(stored), func(text string) bool {
			return text == overlayDoomedComment
		})
		if !reflect.DeepEqual(after, want) {
			t.Fatalf("the confirmed delete left stored comments %v, want %v", after, want)
		}
	}

	// Settled once: the same outcome delivered again is inert, because the panel
	// is no longer waiting on anything.
	settled := panelCommentTexts(panel)
	cursor := panel.Cursor
	deliverHeldOutcome(driver, held)
	if got := panelCommentTexts(panel); !reflect.DeepEqual(got, settled) {
		t.Fatalf("a second delivery re-settled the panel: %v, want the unchanged %v", got, settled)
	}
	if panel.Cursor != cursor {
		t.Fatalf("a second delivery moved the cursor to %d, want %d", panel.Cursor, cursor)
	}
	if got := owner.stored(t, workspace); !reflect.DeepEqual(got, after) {
		t.Fatalf("a second delivery changed stored comments to %v, want %v", got, after)
	}

	press("?")
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("closing Help left the route on %s, want the panel it was raised over", got)
	}
	if got := driver.App().entities.ActiveCommentsPanel(driver.App()); got != panel {
		t.Fatalf("closing Help returned to %p, want the same panel %p", got, panel)
	}
	if refused {
		// The reason is owed to the user who comes back, not only to the status
		// line that expired while Help was up. It is wrapped to the panel's prose
		// width, so the screen is read as a sentence rather than as columns.
		if screen := collapseWhitespace(driver.Render()); !strings.Contains(screen, owner.refusal) {
			t.Fatalf("the panel does not draw the refusal on return:\n%s", driver.Render())
		}
	}

	// The point of settling: the panel still works, without being closed and
	// reopened to clear a submission that finished long ago.
	press("a")
	typeText(overlayAddedComment)
	press("ctrl+s")
	driver.Wait(defaultWaitTimeout)

	if got := panel.StatusMessage; strings.Contains(got, "already in progress") {
		t.Fatalf("the panel still believes a mutation is pending: status = %q", got)
	}
	if panel.AddMode {
		t.Fatalf("the add never completed; the composer is still open:\n%s", driver.Render())
	}
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("the add left the route on %s; the panel was never closed and reopened", got)
	}
	want := append(slices.Clone(after), overlayAddedComment)
	if got := owner.stored(t, workspace); !reflect.DeepEqual(got, want) {
		t.Fatalf("stored comments = %v, want %v", got, want)
	}
}

// confirmDeleteAtCursor walks Delete's only route on this surface — the Space
// sheet, which confirms first — and leaves the confirmation open over the row
// under the cursor.
func confirmDeleteAtCursor(t *testing.T, driver *UITestDriver, press func(string), panel *taskdetail.CommentsModel) {
	t.Helper()
	press("space")
	press("enter")
	if !panel.Confirm.IsActive() || panel.Confirm.Action() != "delete" {
		t.Fatalf("the sheet's Delete row raised confirm %q, want the delete prompt:\n%s",
			panel.Confirm.Action(), driver.Render())
	}
	if err := driver.SendKey("y"); err != nil {
		t.Fatalf("SendKey(y): %v", err)
	}
}

// holdCommentOutcome runs everything the confirmed delete queued except the one
// message that settles the panel, and returns that message instead.
//
// The transaction itself runs here — that is the point. What is held back is the
// delivery, which is the interval the user's `?` lands in, and the only way to
// observe that interval from a driver whose drain is otherwise instantaneous.
func holdCommentOutcome(t *testing.T, driver *UITestDriver) tea.Msg {
	t.Helper()
	queue := []tea.Cmd{driver.pendingCmd}
	driver.pendingCmd = nil

	var held tea.Msg
	steps := 0
	for len(queue) > 0 && steps < maxDrainSteps {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil || isAnimationCmd(cmd) {
			continue
		}
		steps++

		var msg tea.Msg
		select {
		case msg = <-runCmd(cmd):
		case <-time.After(defaultDrainTimeout):
			t.Fatal("a command the confirmed delete queued never finished")
		}
		if isCommentOutcome(msg) {
			if held != nil {
				t.Fatalf("the delete produced a second comment outcome: %T", msg)
			}
			held = msg
			continue
		}
		queue = append(queue, driver.handleMsg(msg)...)
	}
	if held == nil {
		t.Fatal("the confirmed delete produced no comment outcome to hold")
	}
	return held
}

// isCommentOutcome names the four correlated messages a comment commit resolves
// to. They are the vocabulary this file holds back; everything else the delete
// queued is ordinary shell work and runs when it is produced.
func isCommentOutcome(msg tea.Msg) bool {
	switch msg.(type) {
	case taskdetail.CommentAddedMsg, taskdetail.CommentDeletedMsg,
		taskdetail.CommentEditedMsg, taskdetail.CommentMutationFailedMsg:
		return true
	}
	return false
}

// deliverHeldOutcome hands the held message to the shell the way the runtime
// would have, and settles whatever it queues.
func deliverHeldOutcome(driver *UITestDriver, held tea.Msg) {
	driver.dispatch(held)
	driver.drain()
	driver.Wait(defaultWaitTimeout)
}

// commentsPanelUnderTheRoute is the Comments model itself, covered or not.
// ActiveCommentsPanel deliberately answers only when Comments is the current
// view, and the whole subject here is the window where Help is.
func commentsPanelUnderTheRoute(t *testing.T, driver *UITestDriver) *taskdetail.CommentsModel {
	t.Helper()
	panel, _ := driver.App().viewNav.views.Get(presentation.ViewTaskComments).(*taskdetail.CommentsModel)
	if panel == nil {
		t.Fatalf("no Comments model is registered; view = %s", driver.App().currentView())
	}
	return panel
}

// commentOverlayKeys presses and types through the production driver, draining
// each message the way the live program does.
func commentOverlayKeys(t *testing.T, driver *UITestDriver) (press func(string), typeText func(string)) {
	t.Helper()
	press = func(key string) {
		t.Helper()
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("SendKey(%q): %v", key, err)
		}
		driver.drain()
	}
	typeText = func(text string) {
		t.Helper()
		for _, r := range text {
			driver.dispatch(tea.KeyPressMsg{Code: r, Text: string(r)})
			driver.drain()
		}
	}
	return press, typeText
}

// collapseWhitespace folds the wrapping the panel applies to a long refusal, so
// a fragment assertion reads the sentence rather than the column it was fitted
// to.
func collapseWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// taskCommentTextsOnDisk reads one task's stored comments out of the canonical
// document, which is the only place a comment counts as written.
func taskCommentTextsOnDisk(t *testing.T, workspace *uitestWorkspace, title string) []string {
	t.Helper()
	path := filepath.Join(workspace.DataDir, "projects", projectCommentFixtureSlug, "tasks.md")
	content, err := os.ReadFile(path) //nolint:gosec // The path is this test's own workspace.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	document, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, task := range document.Tasks {
		if task == nil || task.Title != title {
			continue
		}
		texts := make([]string, 0, len(task.Comments))
		for _, comment := range task.Comments {
			if comment != nil {
				texts = append(texts, comment.Text)
			}
		}
		return texts
	}
	t.Fatalf("no task titled %q in %s", title, path)
	return nil
}
