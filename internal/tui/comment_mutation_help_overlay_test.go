package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/presentation"
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
