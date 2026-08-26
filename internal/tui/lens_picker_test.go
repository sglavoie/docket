package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// The picker is driven exclusively through key messages here, because the shell
// routing, the nested overlays, and the retention rules only exist together: an
// assertion made by calling the model's own methods would prove nothing about
// which overlay a key actually reaches.

func lensPickerApp(t *testing.T) *AppModel {
	t.Helper()
	model := lensOperationApp(t)
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}
	if model.lensPicker == nil {
		t.Fatalf("openNamedLensPicker left no picker mounted: %v", model.err)
	}
	return model
}

// drainLensPickerCmd runs the overlay protocol to quiescence. Only messages the
// operation-overlay plumbing owns are dispatched: a status or reload message is
// the routed view's business and would drag unrelated machinery into a test
// about overlay nesting.
func drainLensPickerCmd(t *testing.T, model *AppModel, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 64 {
			t.Fatal("lens overlay messages did not settle")
		}
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		switch msg := next().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, []tea.Cmd(msg)...)
		default:
			if handled, follow := model.handleOperationOverlayMessage(msg); handled {
				queue = append(queue, follow)
			}
		}
	}
}

// pressLens sends each named key through the shell's global router, exactly as
// a keystroke arrives in the running app.
func pressLens(t *testing.T, model *AppModel, keys ...string) {
	t.Helper()
	for _, name := range keys {
		key, err := parseUITestKey(name)
		if err != nil {
			t.Fatalf("parse key %q: %v", name, err)
		}
		handled, cmd := model.handleGlobalKeyPress(key)
		if !handled {
			t.Fatalf("key %q was not handled by any overlay", name)
		}
		drainLensPickerCmd(t, model, cmd)
	}
}

func typeLens(t *testing.T, model *AppModel, text string) {
	t.Helper()
	for _, r := range text {
		pressLens(t, model, string(r))
	}
}

func lensPickerRowTitles(t *testing.T, model *AppModel) []string {
	t.Helper()
	if model.lensPicker == nil {
		t.Fatal("no lens picker is mounted")
	}
	items := model.lensPicker.surface.Items()
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.PrimaryText)
	}
	return titles
}

func lensPickerCursorTitle(t *testing.T, model *AppModel) string {
	t.Helper()
	if model.lensPicker == nil {
		t.Fatal("no lens picker is mounted")
	}
	item, ok := model.lensPicker.surface.CurrentItem()
	if !ok {
		t.Fatal("the lens picker has no current row")
	}
	return item.PrimaryText
}

func sheetRowTitles(t *testing.T, model *AppModel) []string {
	t.Helper()
	if model.contextActionSheet == nil {
		t.Fatal("no action sheet is mounted")
	}
	items := model.contextActionSheet.surface.Items()
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.PrimaryText)
	}
	return titles
}

// selectLensRow parks the picker cursor on one saved lens by name so a test can
// name its target without depending on how many keys reach it.
func selectLensRow(t *testing.T, model *AppModel, name string) config.NamedLens {
	t.Helper()
	lens := lensNamed(t, model, name)
	if !model.lensPicker.SelectLens(lens.ID) {
		t.Fatalf("lens %q is not listed in the picker", name)
	}
	return lens
}

// --- population and ordering ------------------------------------------------

func TestLensPickerListsSaveFirstThenLensesAlphabetically(t *testing.T) {
	model := lensPickerApp(t)

	want := []string{lensPickerCreateLabel, "High Priority", "Stale", "Waiting"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("picker rows = %v, want %v", got, want)
	}
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Errorf("opening cursor = %q, want the save row", got)
	}
}

func TestLensPickerOpensOnTheActiveLensAndMarksIt(t *testing.T) {
	model := lensOperationApp(t)
	waiting := lensNamed(t, model, "Waiting")
	if _, err := model.crossOutliner.ApplyNamedLens(waiting); err != nil {
		t.Fatalf("apply lens: %v", err)
	}
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}

	if got := lensPickerCursorTitle(t, model); got != "Waiting" {
		t.Fatalf("cursor = %q, want the active lens", got)
	}
	item, _ := model.lensPicker.surface.CurrentItem()
	if item.PrimaryMarker == "" {
		t.Error("the active lens row carries no marker")
	}
}

// --- navigation -------------------------------------------------------------

func TestLensPickerNavigationKeysMoveTheCursor(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "j")
	if got := lensPickerCursorTitle(t, model); got != "High Priority" {
		t.Fatalf("after j cursor = %q", got)
	}
	pressLens(t, model, "down")
	if got := lensPickerCursorTitle(t, model); got != "Stale" {
		t.Fatalf("after down cursor = %q", got)
	}
	pressLens(t, model, "k", "up")
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Fatalf("after k/up cursor = %q", got)
	}
	pressLens(t, model, "end")
	if got := lensPickerCursorTitle(t, model); got != "Waiting" {
		t.Fatalf("after end cursor = %q", got)
	}
	pressLens(t, model, "home")
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Fatalf("after home cursor = %q", got)
	}
	pressLens(t, model, "pgdown")
	if got := lensPickerCursorTitle(t, model); got == lensPickerCreateLabel {
		t.Error("pgdown left the cursor on the save row")
	}
	pressLens(t, model, "pgup")
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Fatalf("after pgup cursor = %q", got)
	}
}

// --- search cascade ---------------------------------------------------------

func TestLensPickerSearchNarrowsLensesAndKeepsSaveAvailable(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "/")
	typeLens(t, model, "stal")
	want := []string{lensPickerCreateLabel, "Stale"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("filtered rows = %v, want %v", got, want)
	}

	typeLens(t, model, "zzz")
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, []string{lensPickerCreateLabel}) {
		t.Fatalf("no-match rows = %v, want the save row alone", got)
	}
	if state := model.lensPicker.stateLine(); !strings.Contains(state, "No matching lenses") {
		t.Errorf("no-match state line = %q", state)
	}

	pressLens(t, model, "backspace", "backspace", "backspace")
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("after backspace rows = %v, want %v", got, want)
	}
}

func TestLensPickerEscapeClearsSearchThenLeavesSearchThenCloses(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "stale")

	pressLens(t, model, "esc")
	if model.lensPicker == nil {
		t.Fatal("the first Esc closed the picker instead of clearing the query")
	}
	if model.lensPicker.query != "" || !model.lensPicker.searching {
		t.Fatalf("first Esc left query=%q searching=%v", model.lensPicker.query, model.lensPicker.searching)
	}

	pressLens(t, model, "esc")
	if model.lensPicker == nil {
		t.Fatal("the second Esc closed the picker instead of leaving search")
	}
	if model.lensPicker.searching {
		t.Error("the second Esc left the picker in search mode")
	}

	pressLens(t, model, "esc")
	if model.lensPicker != nil {
		t.Error("the third Esc left the picker mounted")
	}
}

func TestLensPickerSearchEnterOnSaveRowStillOpensTheNamePrompt(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "zzz")
	// Enter leaves search with the query retained, so the no-match narrowing is
	// still in force and the save row is the only survivor — which is the whole
	// point of exempting it from the filter.
	pressLens(t, model, "enter")
	if model.lensPicker.searching || model.lensPicker.query != "zzz" {
		t.Fatalf("leaving search dropped the query: %q", model.lensPicker.query)
	}
	pressLens(t, model, "enter")

	if model.operationTextPrompt == nil {
		t.Fatal("Enter on the save row raised no name prompt")
	}
	if model.lensPicker == nil {
		t.Error("the picker must stay mounted beneath the name prompt")
	}
}

// --- apply ------------------------------------------------------------------

func TestLensPickerEnterAppliesLensAndClosesThePicker(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Waiting")

	pressLens(t, model, "enter")

	if model.lensPicker != nil {
		t.Error("applying a lens left the picker mounted")
	}
	if !model.crossOutliner.hasActiveLens() {
		t.Fatal("applying a lens installed nothing")
	}
	if got := model.crossOutliner.activeLens.name; got != "Waiting" {
		t.Errorf("active lens = %q, want Waiting", got)
	}
}

// --- create -----------------------------------------------------------------

func TestLensPickerCreateClosesEveryOverlayAndLeavesTheNewLensActive(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "enter")
	if model.operationTextPrompt == nil {
		t.Fatal("Enter on the save row raised no name prompt")
	}
	typeLens(t, model, "Focus")
	pressLens(t, model, "enter")

	if model.lensPicker != nil || model.contextActionSheet != nil ||
		model.operationTextPrompt != nil || model.operationValuePicker != nil {
		t.Error("creating a lens left a lens overlay mounted")
	}
	if model.Overlays().Active() {
		t.Error("creating a lens left the overlay manager active")
	}
	saved := lensNamed(t, model, "Focus")
	if !model.crossOutliner.hasActiveLens() ||
		model.crossOutliner.activeLens.lensID != saved.ID {
		t.Error("the new lens is not the active one")
	}
}

func TestLensPickerCreateCancelReturnsToThePicker(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "enter")
	pressLens(t, model, "esc")

	if model.operationTextPrompt != nil {
		t.Error("Esc left the name prompt mounted")
	}
	if model.lensPicker == nil {
		t.Fatal("canceling the name prompt closed the picker")
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("canceling wrote %d lenses", got)
	}
}

func TestLensPickerCreateFailureStaysOnThePickerWithTheReason(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "enter")
	typeLens(t, model, "Stale")
	pressLens(t, model, "enter")

	if model.lensPicker == nil {
		t.Fatal("a refused save closed the picker")
	}
	if notice := model.lensPicker.Notice(); !strings.Contains(notice, "already exists") {
		t.Errorf("picker notice = %q, want the duplicate-name reason", notice)
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("a refused save wrote %d lenses", got)
	}
	// The reason is retained on the surface that raised it, so the same row is
	// still one Enter away from a corrected retry.
	pressLens(t, model, "enter")
	if model.operationTextPrompt == nil {
		t.Fatal("retrying the save raised no name prompt")
	}
}

// --- management sheet -------------------------------------------------------

func TestLensPickerSpaceOpensManagementActionsOverThePicker(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Stale")

	pressLens(t, model, "space")

	if model.contextActionSheet == nil {
		t.Fatal("Space raised no action sheet")
	}
	if model.lensPicker == nil {
		t.Error("the picker must stay mounted beneath the action sheet")
	}
	want := []string{"Rename lens", "Replace from current view", "Delete lens"}
	if got := sheetRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("sheet rows = %v, want %v", got, want)
	}
}

func TestLensPickerSpaceOnTheSaveRowRaisesNoSheet(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "space")

	if model.contextActionSheet != nil {
		t.Error("the save row named a management target it does not have")
	}
	if model.lensPicker == nil {
		t.Fatal("Space on the save row closed the picker")
	}
}

func TestLensPickerSheetEscapeReturnsToThePicker(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Stale")
	pressLens(t, model, "space")

	pressLens(t, model, "esc")

	if model.contextActionSheet != nil {
		t.Error("Esc left the action sheet mounted")
	}
	if model.lensPicker == nil {
		t.Fatal("Esc on the sheet closed the picker beneath it")
	}
	if got := lensPickerCursorTitle(t, model); got != "Stale" {
		t.Errorf("cursor = %q, want the row the sheet acted on", got)
	}
}

// --- rename -----------------------------------------------------------------

func TestLensRenameReturnsToThePickerWithTheSameLensSelected(t *testing.T) {
	model := lensPickerApp(t)
	target := selectLensRow(t, model, "Waiting")
	pressLens(t, model, "space")
	pressLens(t, model, "enter") // Rename

	if model.operationTextPrompt == nil {
		t.Fatal("Rename raised no name prompt")
	}
	// The prompt prefills the current name, so clearing it is what a real rename
	// types over.
	for range len("Waiting") {
		pressLens(t, model, "backspace")
	}
	typeLens(t, model, "Blocked")
	pressLens(t, model, "enter")

	if model.contextActionSheet != nil || model.operationTextPrompt != nil {
		t.Error("a completed rename left a nested overlay mounted")
	}
	if model.lensPicker == nil {
		t.Fatal("a completed rename closed the picker")
	}
	want := []string{lensPickerCreateLabel, "Blocked", "High Priority", "Stale"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("rows after rename = %v, want %v", got, want)
	}
	if got := lensPickerCursorTitle(t, model); got != "Blocked" {
		t.Errorf("cursor = %q, want the renamed lens", got)
	}
	if renamed := lensNamed(t, model, "Blocked"); renamed.ID != target.ID {
		t.Error("the rename minted a new identity")
	}
}

func TestLensRenameCancelReturnsToTheActionSheet(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Waiting")
	pressLens(t, model, "space")
	pressLens(t, model, "enter")

	pressLens(t, model, "esc")

	if model.operationTextPrompt != nil {
		t.Error("Esc left the rename prompt mounted")
	}
	if model.contextActionSheet == nil {
		t.Fatal("canceling the rename must return to the action sheet")
	}
	if model.lensPicker == nil {
		t.Error("canceling the rename closed the picker")
	}
	if _, err := lensNamedOrErr(model, "Waiting"); err != nil {
		t.Errorf("canceling the rename changed the record: %v", err)
	}
}

func TestLensRenameFailureStaysOnTheActionSheet(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Waiting")
	pressLens(t, model, "space")
	pressLens(t, model, "enter")
	for range len("Waiting") {
		pressLens(t, model, "backspace")
	}
	typeLens(t, model, "Stale")
	pressLens(t, model, "enter")

	if model.contextActionSheet == nil {
		t.Fatal("a refused rename closed the action sheet")
	}
	if notice := model.contextActionSheet.notice; !strings.Contains(notice, "already exists") {
		t.Errorf("sheet notice = %q, want the duplicate-name reason", notice)
	}
	if _, err := lensNamedOrErr(model, "Waiting"); err != nil {
		t.Errorf("a refused rename changed the record: %v", err)
	}
}

// --- replace ----------------------------------------------------------------

func TestLensReplaceConfirmCancelReturnsToTheActionSheet(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Waiting")
	pressLens(t, model, "space")
	pressLens(t, model, "j")
	pressLens(t, model, "enter") // Replace from current view

	if !model.operationConfirm.IsActive() {
		t.Fatal("Replace raised no confirmation")
	}

	pressLens(t, model, "esc")

	if model.operationConfirm.IsActive() {
		t.Error("Esc left the confirmation active")
	}
	if model.contextActionSheet == nil {
		t.Fatal("canceling the confirmation must return to the action sheet")
	}
	if model.lensPicker == nil {
		t.Error("canceling the confirmation closed the picker")
	}
	if saved := lensNamed(t, model, "Waiting"); !saved.Tasks.StatesSet {
		t.Error("a canceled replace overwrote the saved definition")
	}
}

func TestLensReplaceConfirmedReturnsToTheRefreshedPicker(t *testing.T) {
	model := lensPickerApp(t)
	target := selectLensRow(t, model, "Waiting")
	// Give the live view a facet the saved definition does not carry, so the
	// replacement is observable.
	model.crossOutliner.filter.Pinned = true
	pressLens(t, model, "space")
	pressLens(t, model, "j")
	pressLens(t, model, "enter")
	pressLens(t, model, "y")

	if model.contextActionSheet != nil || model.operationConfirm.IsActive() {
		t.Error("a completed replace left a nested overlay mounted")
	}
	if model.lensPicker == nil {
		t.Fatal("a completed replace closed the picker")
	}
	if got := lensPickerCursorTitle(t, model); got != "Waiting" {
		t.Errorf("cursor = %q, want the replaced lens", got)
	}
	saved := lensNamed(t, model, "Waiting")
	if saved.ID != target.ID {
		t.Error("the replace minted a new identity")
	}
	if !saved.Tasks.Pinned {
		t.Error("the replacement did not capture the live view")
	}
}

// --- delete -----------------------------------------------------------------

func TestLensDeleteReturnsToTheNearestSurvivingRow(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Stale")
	pressLens(t, model, "space")
	pressLens(t, model, "j", "j")
	pressLens(t, model, "enter") // Delete lens
	if !model.operationConfirm.IsActive() {
		t.Fatal("Delete raised no confirmation")
	}
	pressLens(t, model, "y")

	if model.lensPicker == nil {
		t.Fatal("a completed delete closed the picker")
	}
	want := []string{lensPickerCreateLabel, "High Priority", "Waiting"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("rows after delete = %v, want %v", got, want)
	}
	if got := lensPickerCursorTitle(t, model); got != "Waiting" {
		t.Errorf("cursor = %q, want the nearest surviving row", got)
	}
}

func TestLensDeleteOfTheActiveLensRestoresTheBaseBeneathThePicker(t *testing.T) {
	model := lensOperationApp(t)
	baseMode := model.crossOutliner.primaryMode
	waiting := lensNamed(t, model, "Waiting")
	if _, err := model.crossOutliner.ApplyNamedLens(waiting); err != nil {
		t.Fatalf("apply lens: %v", err)
	}
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}
	selectLensRow(t, model, "Waiting")
	pressLens(t, model, "space")
	pressLens(t, model, "j", "j")
	pressLens(t, model, "enter")
	pressLens(t, model, "y")

	if model.lensPicker == nil {
		t.Fatal("deleting the active lens closed the picker")
	}
	if model.crossOutliner.hasActiveLens() {
		t.Error("the deleted lens is still active")
	}
	if got := model.crossOutliner.primaryMode; got != baseMode {
		t.Errorf("restored mode = %v, want the pre-lens %v", got, baseMode)
	}
	if got := lensPickerRowTitles(t, model); slices.Contains(got, "Waiting") {
		t.Errorf("rows after delete = %v, still list the deleted lens", got)
	}
}

func TestLensDeleteConfirmCancelReturnsToTheActionSheet(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Stale")
	pressLens(t, model, "space")
	pressLens(t, model, "j", "j")
	pressLens(t, model, "enter")
	pressLens(t, model, "n")

	if model.contextActionSheet == nil {
		t.Fatal("canceling the confirmation must return to the action sheet")
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("a canceled delete left %d lenses", got)
	}
	if model.pendingOperationRequest.ID != "" {
		t.Error("a canceled confirmation left a pending request")
	}
}

// --- rendering --------------------------------------------------------------

func TestLensPickerRendersEmptyNoMatchAndOversizedRows(t *testing.T) {
	model := lensPickerApp(t)

	long := strings.Repeat("Quarterly Planning ", 12)
	if err := model.lensPicker.Refresh(NamedLensPickerSnapshot{
		Lenses: config.NamedLensList{{
			ID:    "11111111-1111-4111-8111-111111111111",
			Name:  long,
			Mode:  config.LensModeAll,
			Tasks: config.LensTaskFilters{Stale: true, Deferred: true, Pinned: true},
			Sort:  config.LensSort{Field: config.LensSortDefault},
		}},
		CurrentSummary: strings.Repeat("summary segment · ", 20),
	}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	for _, width := range []int{minTerminalWidth, 80, 200} {
		rendered := ansi.Strip(model.lensPicker.render(width))
		assertNoLensPickerOverflow(t, rendered, width)
	}
	// The minimum supported terminal must still show the modal's own chrome,
	// which is what renderWithin's reservation buys.
	rendered := ansi.Strip(model.lensPicker.renderWithin(minTerminalWidth, minTerminalHeight))
	assertNoLensPickerOverflow(t, rendered, minTerminalWidth)
	if lines := strings.Count(rendered, "\n") + 1; lines > minTerminalHeight {
		t.Errorf("minimum-terminal render is %d lines, taller than %d", lines, minTerminalHeight)
	}

	if err := model.lensPicker.Refresh(NamedLensPickerSnapshot{}); err != nil {
		t.Fatalf("refresh empty: %v", err)
	}
	empty := ansi.Strip(model.lensPicker.render(80))
	if !strings.Contains(empty, "No saved lenses yet") {
		t.Errorf("empty render names no empty state:\n%s", empty)
	}
	if !strings.Contains(empty, "Save current view") {
		t.Errorf("empty render dropped the save row:\n%s", empty)
	}
}

func assertNoLensPickerOverflow(t *testing.T, rendered string, width int) {
	t.Helper()
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line is %d columns wide at width %d: %q", got, width, line)
		}
	}
}

// --- teardown ---------------------------------------------------------------

func TestLensPickerIsClearedByNavigationCollapse(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Stale")
	pressLens(t, model, "space")
	pressLens(t, model, "j", "j")
	pressLens(t, model, "enter")

	model.clearTransientNavigationState()

	if model.lensPicker != nil || model.contextActionSheet != nil ||
		model.operationConfirm.IsActive() {
		t.Error("a navigation collapse left a lens overlay mounted")
	}
	if model.pendingOperationRequest.ID != "" {
		t.Error("a navigation collapse left a pending operation")
	}
	if model.Overlays().Active() || model.Overlays().HasActiveTextInput() {
		t.Error("the overlay manager still reports the picker")
	}
}

func TestLensPickerDoesNotSwallowQuit(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "st")

	// Quit outranks every overlay, search mode included: the picker must not
	// absorb Ctrl+C as one more character.
	handled, cmd := model.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !handled || cmd == nil {
		t.Fatalf("ctrl+c handled=%v cmd=%v, want the quit command", handled, cmd)
	}
	if model.lensPicker.query != "st" {
		t.Errorf("ctrl+c was typed into the search query: %q", model.lensPicker.query)
	}
}

func TestLensPickerCountsAsAnActiveOverlayAndOwnsTextInput(t *testing.T) {
	model := lensPickerApp(t)

	if !model.Overlays().Active() {
		t.Error("a mounted picker is not reported as an active overlay")
	}
	if !model.Overlays().HasActiveTextInput() {
		t.Error("a mounted picker does not claim text input, so root keys would leak into it")
	}
	model.Overlays().ClearAll()
	if model.lensPicker != nil {
		t.Error("ClearAll left the picker mounted")
	}
}

// --- summary ----------------------------------------------------------------

func TestNamedLensSummaryNamesOnlyTheAxesTheLensSets(t *testing.T) {
	summary := namedLensSummary(config.NamedLens{
		Mode:  config.LensModeAll,
		Query: "#auth",
		Tasks: config.LensTaskFilters{
			States:           []string{string(models.StateWaiting)},
			StatesSet:        true,
			Priority:         models.PriorityHigh,
			Tags:             []string{"api"},
			DueDate:          config.LensDueAll,
			Hierarchy:        config.LensHierarchyAll,
			Stale:            true,
			CompletionAgeSet: true,
			CompletionAge:    config.LensCompletionAgeAll,
		},
		Projects: config.LensProjectFilters{
			Tags:         []string{"platform"},
			ParkedMode:   config.LensPopulationInclude,
			ArchivedMode: config.LensPopulationExclude,
		},
		Sort: config.LensSort{Field: config.LensSortPriority, Ascending: true},
	})

	for _, want := range []string{
		"/#auth", "states: waiting", "!1", "#api", "done: ", "stale",
		"%platform", "parked: " + config.LensPopulationInclude, "sort: ",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q is missing %q", summary, want)
		}
	}
	for _, unwanted := range []string{"due:", "tree:", "archived:", "deferred", "pinned", "%untagged"} {
		if strings.Contains(summary, unwanted) {
			t.Errorf("summary %q names the defaulted axis %q", summary, unwanted)
		}
	}
}

func TestNamedLensSummaryOmitsTheDefaultSort(t *testing.T) {
	summary := namedLensSummary(config.NamedLens{
		Mode: config.LensModeAll,
		Sort: config.LensSort{Field: config.LensSortDefault},
	})
	if strings.Contains(summary, "sort:") {
		t.Errorf("summary %q names the default sort", summary)
	}
}

// --- helpers ----------------------------------------------------------------

func lensNamedOrErr(model *AppModel, name string) (config.NamedLens, error) {
	machine := model.runtime.Machine()
	if machine == nil {
		return config.NamedLens{}, errors.New("machine-local state is unavailable")
	}
	for _, lens := range machine.NamedLenses {
		if lens.Name == name {
			return lens, nil
		}
	}
	return config.NamedLens{}, fmt.Errorf("no saved lens named %q", name)
}
