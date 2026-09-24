package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

// This file is the other half of `S`: everything the key must decline, and
// everything it must leave alone while declining it. A promotion consumes the
// row it was pressed on, so a refusal that moved the cursor, dropped a fold, or
// converted a neighbor would be indistinguishable from a promotion the user
// never asked for.

// holdPromotion presses `S` and returns the commit it queued without running it,
// which is the only way to stand inside the interval between a frozen checklist
// and the transaction that checks it.
func holdPromotion(t *testing.T, driver *UITestDriver) tea.Cmd {
	t.Helper()
	if err := driver.SendKey("S"); err != nil {
		t.Fatalf("SendKey(S): %v", err)
	}
	held := driver.pendingCmd
	driver.pendingCmd = nil
	if held == nil {
		t.Fatalf("S queued no commit:\n%s", driver.Render())
	}
	return held
}

// runHeldPromotion delivers one held commit through the shell exactly as the
// runtime would have.
func runHeldPromotion(driver *UITestDriver, held tea.Cmd) {
	driver.processCmds([]tea.Cmd{held}, driver.drainOpts())
	driver.drain()
}

// TestChecklistPromotionRefusesTheShiftedNeighborOnARepeatedPress is the reason
// the whole ordered checklist travels with the gesture. Two presses land before
// either commit does; the first consumes the item, and the second — frozen
// against a checklist that no longer exists — must refuse by name rather than
// convert whatever item slid into the index it named.
func TestChecklistPromotionRefusesTheShiftedNeighborOnARepeatedPress(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)

	first := holdPromotion(t, driver)
	// The row has not moved yet, because nothing has committed: this is the
	// double press a fast hand actually produces.
	second := holdPromotion(t, driver)

	runHeldPromotion(driver, first)
	runHeldPromotion(driver, second)

	view = driver.App().crossOutliner
	want := []string{checklistFixtureFirstItem, checklistFixtureThirdItem}
	if got := checklistItemTexts(t, view, checklistFixtureFoldTitle); !slices.Equal(got, want) {
		t.Fatalf("the owner's checklist is %q, want exactly one promotion to have landed: %q", got, want)
	}
	if childSurvives(driver.App(), checklistFixtureThirdItem) {
		t.Error("the second press converted the item that slid into the index the first one consumed")
	}
	err := driver.App().feedback.err
	if err == nil {
		t.Fatalf("the second press was silently dropped; nothing named the refusal:\n%s", driver.Render())
	}
	if !strings.Contains(err.Error(), "changed since it was read") {
		t.Errorf("the refusal says %q, want it to name the checklist that moved underneath it", err)
	}
}

// TestChecklistPromotionRefusesAnOwnerThatDisappeared covers the perturbation no
// frozen list can see: the owner itself is gone by the time the commit runs. The
// promotion is refused whole, nothing is written, and the outline the user is
// looking at is left exactly as it was.
func TestChecklistPromotionRefusesAnOwnerThatDisappeared(t *testing.T) {
	driver, workspace, view := checklistFixtureApp(t)
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	cursor := view.Cursor

	held := holdPromotion(t, driver)

	owner, projectID := workspaceTaskTitled(t, driver.App(), checklistFixtureFoldTitle)
	if _, err := externalProjectCommentWriter(t, workspace).Delete(app.DeleteRequest{
		Locator: app.TaskLocator{ProjectID: projectID, TaskID: owner.ID},
		Confirm: true,
	}); err != nil {
		t.Fatalf("the external writer could not delete the owner: %v", err)
	}

	runHeldPromotion(driver, held)

	if driver.App().feedback.err == nil {
		t.Fatalf("promoting an item of a deleted owner reported nothing:\n%s", driver.Render())
	}
	if childSurvives(driver.App(), checklistFixtureSecondItem) {
		t.Error("the refused promotion still wrote its child")
	}
	view = driver.App().crossOutliner
	if view.Cursor != cursor {
		t.Errorf("cursor = %d after a refused promotion, want the row it was pressed on (%d)", view.Cursor, cursor)
	}
	if got := cursorTaskTitle(t, view); got != "checklist item "+checklistFixtureSecondItem {
		t.Errorf("the cursor rests on %q, want the item the refusal left alone", got)
	}
}

// TestChecklistPromotionSettlesUnderHelp holds the success in the one interval
// the user can cover it: `?` is an overlay, so the Outliner underneath is still
// the model that owes the reveal. The child lands, the cursor beneath moves to
// it, and Help stays where the user put it.
func TestChecklistPromotionSettlesUnderHelp(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)

	held := holdPromotion(t, driver)
	if err := driver.SendKey("?"); err != nil {
		t.Fatalf("SendKey(?): %v", err)
	}
	driver.drain()
	if got := driver.App().currentView(); got != presentation.ViewHelp {
		t.Fatalf("? left the route on %s, want Help over the Outliner", got)
	}

	runHeldPromotion(driver, held)

	if got := driver.App().currentView(); got != presentation.ViewHelp {
		t.Fatalf("the promotion moved the route to %s, want Help still visible", got)
	}
	if !childSurvives(driver.App(), checklistFixtureSecondItem) {
		t.Fatal("the promotion that completed under Help committed nothing")
	}
	view = driver.App().crossOutliner
	if got := cursorTaskTitle(t, view); got != checklistFixtureSecondItem {
		t.Errorf("the covered Outliner's cursor rests on %q, want the committed child", got)
	}

	// Closing Help lands on the reveal rather than on a view that has to catch up.
	if err := driver.SendKey("?"); err != nil {
		t.Fatalf("SendKey(?): %v", err)
	}
	driver.drain()
	if !strings.Contains(driver.Render(), checklistFixtureSecondItem) {
		t.Errorf("the child is not on screen after Help closed:\n%s", driver.Render())
	}
}

// TestChecklistPromotionRefusesEveryRowThatNamesNoItem is the applicability
// contract. `S` is a root key on a surface whose rows are mostly not checklist
// items, so every refusal here is one a fallback to a nearby task would have
// turned into a conversion nobody asked for.
func TestChecklistPromotionRefusesEveryRowThatNamesNoItem(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	appModel := driver.App()
	descriptor, ok := appModel.GetOperationRegistry().Descriptor(operation.OperationChecklistPromoteItem)
	if !ok {
		t.Fatal("Make Subtask is not registered")
	}

	for _, title := range []string{
		checklistFixturePlainTitle,  // no checklist at all
		checklistFixtureFoldTitle,   // a populated checklist, but the owner row
		checklistFixtureParentTitle, // a populated checklist and a child
	} {
		seatCursorOnTaskTitled(t, view, title)
		if descriptor.Applicable(appModel.operationCommandContext()) {
			t.Errorf("S applied on the task row %q, which names no checklist item", title)
		}
	}

	seatCursorOnRow(t, view, "section header", func(r outliner.OutlinerRow) bool { return r.IsSectionHeader })
	if descriptor.Applicable(appModel.operationCommandContext()) {
		t.Error("S applied on a section header")
	}
	// The project header is the folded group's summary, so reaching it means
	// folding: an expanded group draws no header for the cursor to sit on.
	onProjectHeaderCursor(t, view, func() {
		if descriptor.Applicable(appModel.operationCommandContext()) {
			t.Error("S applied on a project header")
		}
	})

	// The one row it does answer from, so the refusals above are not vacuous.
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	if !descriptor.Applicable(appModel.operationCommandContext()) {
		t.Fatal("S refused the checklist item row it exists for")
	}

	// A visual selection names task rows, so answering it with one hidden
	// conversion is worse than refusing: promotion steps aside entirely.
	context := appModel.operationCommandContext()
	context.Selection.Visual = true
	if descriptor.Applicable(context) {
		t.Error("S applied under a visual selection")
	}
}

// TestChecklistPromotionKeyIsInertOnATaskRow is the same refusal driven through
// the real key path rather than through the descriptor: a refused `S` writes
// nothing and moves nothing, so it reads as an unbound key rather than as a
// mutation that quietly failed.
func TestChecklistPromotionKeyIsInertOnATaskRow(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)
	cursor := view.Cursor
	before := checklistItemTexts(t, view, checklistFixtureFoldTitle)

	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "S"}})

	view = driver.App().crossOutliner
	if got := checklistItemTexts(t, view, checklistFixtureFoldTitle); !slices.Equal(got, before) {
		t.Errorf("S on a task row changed the checklist to %q, want the untouched %q", got, before)
	}
	if view.Cursor != cursor {
		t.Errorf("cursor = %d after a refused S, want %d", view.Cursor, cursor)
	}
	if err := driver.App().feedback.err; err != nil {
		t.Errorf("S on a task row raised %v, want an unbound key's silence", err)
	}
}

// TestPromotionKeyStaysLiteralInEveryTextSurface keeps `S` a grammar key only
// where there is a grammar. Every buffer the Outliner can open has to take the
// character, because a capital S is an ordinary letter in an ordinary sentence.
func TestPromotionKeyStaysLiteralInEveryTextSurface(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)

	t.Run("composer", func(t *testing.T) {
		const title = "Ship the S release"
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "a"},
			{Type: UITestActionText, Text: title},
		})
		composer, ok := driver.App().viewNav.views.Get(presentation.ViewTaskComposer).(*taskform.TaskComposerModel)
		if !ok {
			t.Fatalf("a opened %T, want the composer", driver.App().viewNav.views.Get(presentation.ViewTaskComposer))
		}
		if got := composer.Input.Value(); got != title {
			t.Fatalf("the composer holds %q, want the literal %q", got, title)
		}
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "enter"},
			{Type: UITestActionWait},
		})
		if !childSurvives(driver.App(), title) {
			t.Fatalf("the captured task is not in the workspace:\n%s", driver.Render())
		}
	})

	t.Run("inline row editor", func(t *testing.T) {
		view = driver.App().crossOutliner
		seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "e"},
			{Type: UITestActionText, Text: " S"},
		})
		view = driver.App().crossOutliner
		if !view.QuickEdit.IsActive() {
			t.Fatalf("e did not open the row editor:\n%s", driver.Render())
		}
		if got := view.QuickEdit.Input.Value(); !strings.HasSuffix(got, " S") {
			t.Fatalf("the row editor holds %q, want the typed S at its end", got)
		}
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "esc"}})
	})

	t.Run("checklist item prompt", func(t *testing.T) {
		const item = "Sign the S waiver"
		view = driver.App().crossOutliner
		seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "c"},
			{Type: UITestActionText, Text: item},
		})
		prompt := driver.App().operations.TextPrompt
		if prompt == nil {
			t.Fatalf("c opened no item prompt:\n%s", driver.Render())
		}
		if got := prompt.Value(); got != item {
			t.Fatalf("the item prompt holds %q, want the literal %q", got, item)
		}
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "enter"},
			{Type: UITestActionWait},
		})
		if got := checklistItemTexts(t, driver.App().crossOutliner, checklistFixturePlainTitle); !slices.Contains(got, item) {
			t.Fatalf("the committed checklist is %q, want it to carry %q", got, item)
		}
	})
}

// TestChecklistBindingsSurviveThePromotionKey walks every key the checklist row
// grammar already owned. A new binding on a shared surface is exactly the change
// that shadows a neighbor, and the item grammar is small enough to check whole.
func TestChecklistBindingsSurviveThePromotionKey(t *testing.T) {
	driver, _, view := checklistFixtureApp(t)
	expandChecklistOf(t, view, checklistFixtureFoldTitle)

	// x ticks the item under the cursor.
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "x"},
		{Type: UITestActionWait},
	})
	if !checklistItemDone(t, driver.App(), checklistFixtureFoldTitle, checklistFixtureSecondItem) {
		t.Error("x no longer ticks the checklist item under the cursor")
	}

	// K and J move it up and back down.
	view = driver.App().crossOutliner
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "K"},
		{Type: UITestActionWait},
	})
	if got := checklistItemTexts(t, driver.App().crossOutliner, checklistFixtureFoldTitle)[0]; got != checklistFixtureSecondItem {
		t.Errorf("after K the first item is %q, want the moved %q", got, checklistFixtureSecondItem)
	}
	view = driver.App().crossOutliner
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "J"},
		{Type: UITestActionWait},
	})
	if got := checklistItemTexts(t, driver.App().crossOutliner, checklistFixtureFoldTitle)[1]; got != checklistFixtureSecondItem {
		t.Errorf("after J the second item is %q, want the moved %q", got, checklistFixtureSecondItem)
	}

	// Ctrl+J and Ctrl+K carry it to the far ends.
	view = driver.App().crossOutliner
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+j"},
		{Type: UITestActionWait},
	})
	texts := checklistItemTexts(t, driver.App().crossOutliner, checklistFixtureFoldTitle)
	if got := texts[len(texts)-1]; got != checklistFixtureSecondItem {
		t.Errorf("after ctrl+j the last item is %q, want the moved %q", got, checklistFixtureSecondItem)
	}
	view = driver.App().crossOutliner
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+k"},
		{Type: UITestActionWait},
	})
	if got := checklistItemTexts(t, driver.App().crossOutliner, checklistFixtureFoldTitle)[0]; got != checklistFixtureSecondItem {
		t.Errorf("after ctrl+k the first item is %q, want the moved %q", got, checklistFixtureSecondItem)
	}

	// e edits it in place.
	view = driver.App().crossOutliner
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	const edited = "Name the on-call owner and their backup"
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "e"},
		{Type: UITestActionText, Text: " and their backup"},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	if got := checklistItemTexts(t, driver.App().crossOutliner, checklistFixtureFoldTitle); !slices.Contains(got, edited) {
		t.Errorf("after e the checklist is %q, want it to carry %q", got, edited)
	}

	// Space still lists Edit Item and Delete Item, and Make Subtask is not among
	// them: a direct key is the whole route, and the sheet is where a second one
	// would earn nothing.
	view = driver.App().crossOutliner
	seatCursorOnChecklistItem(t, view, edited)
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "space"}})
	titles := sheetRowTitles(t, driver.App())
	if !slices.Contains(titles, "Edit Item") || !slices.Contains(titles, "Delete Item") {
		t.Errorf("the checklist sheet lists %v, want both Edit Item and Delete Item", titles)
	}
	if slices.Contains(titles, "Make Subtask") {
		t.Errorf("the checklist sheet lists %v, want Make Subtask to stay direct-only", titles)
	}
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "esc"}})
}

// checklistItemDone reports whether one committed item is ticked, read off the
// reloaded workspace rather than off the glyph the row happens to draw.
func checklistItemDone(t *testing.T, app *AppModel, owner, text string) bool {
	t.Helper()
	task, _ := workspaceTaskTitled(t, app, owner)
	for _, item := range task.Checklist {
		if item.Text == text {
			return item.Done
		}
	}
	t.Fatalf("task %q carries no checklist item %q", owner, text)
	return false
}
