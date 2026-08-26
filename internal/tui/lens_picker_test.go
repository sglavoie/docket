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

func TestLensPickerListsBothActionRowsFirstThenLensesAlphabetically(t *testing.T) {
	model := lensPickerApp(t)

	want := []string{
		lensPickerCreateLabel, lensPickerAddTemplateLabel,
		"High Priority", "Stale", "Waiting",
	}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("picker rows = %v, want %v", got, want)
	}
	if got := lensPickerCursorTitle(t, model); got != lensPickerCreateLabel {
		t.Errorf("opening cursor = %q, want the save row", got)
	}
	items := model.lensPicker.surface.Items()
	if got := items[1].ID; got != lensPickerAddTemplateItemID {
		t.Errorf("second row ID = %q, want %q", got, lensPickerAddTemplateItemID)
	}
	if got := items[1].SecondaryText; got != lensPickerAddTemplateSecondary {
		t.Errorf("add row secondary = %q, want %q", got, lensPickerAddTemplateSecondary)
	}
	for _, alias := range []string{"template", "preset", "built-in view"} {
		if !containsString(items[1].Aliases, alias) {
			t.Errorf("add row aliases = %v, want %q among them", items[1].Aliases, alias)
		}
	}
	if got := items[1].OperationIDs; len(got) != 1 || got[0] != operationLensAddTemplate {
		t.Errorf("add row operations = %v, want just %q", got, operationLensAddTemplate)
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

	// One move down reaches the second action row, not the first saved lens:
	// both fixed rows sit above the population.
	pressLens(t, model, "j")
	if got := lensPickerCursorTitle(t, model); got != lensPickerAddTemplateLabel {
		t.Fatalf("after j cursor = %q", got)
	}
	pressLens(t, model, "down")
	if got := lensPickerCursorTitle(t, model); got != "High Priority" {
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

func TestLensPickerSearchNarrowsLensesAndKeepsBothActionRowsAvailable(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "/")
	typeLens(t, model, "stal")
	want := []string{lensPickerCreateLabel, lensPickerAddTemplateLabel, "Stale"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("filtered rows = %v, want %v", got, want)
	}

	typeLens(t, model, "zzz")
	actions := []string{lensPickerCreateLabel, lensPickerAddTemplateLabel}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, actions) {
		t.Fatalf("no-match rows = %v, want the two action rows alone", got)
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

// --- add built-in view ------------------------------------------------------

// selectGalleryRow parks the gallery cursor on one template by its stable ID, so
// a test names its target without depending on the alphabetical position it
// happens to occupy.
func selectGalleryRow(t *testing.T, model *AppModel, id string) {
	t.Helper()
	if model.operationValuePicker == nil {
		t.Fatal("no gallery is mounted")
	}
	if !model.operationValuePicker.surface.SetCursorToID(lensTemplateItemID(id)) {
		t.Fatalf("the gallery has no row for template %s", id)
	}
}

// openAddTemplateGalleryFromPicker walks the real nesting: the main picker's
// second action row raises the value step, and the gallery opens above it.
func openAddTemplateGalleryFromPicker(t *testing.T, model *AppModel) {
	t.Helper()
	if !model.lensPicker.surface.SetCursorToID(lensPickerAddTemplateItemID) {
		t.Fatal("the picker has no Add built-in view row")
	}
	pressLens(t, model, "enter")
	if model.operationValuePicker == nil {
		t.Fatal("Enter on the add row raised no gallery")
	}
	if model.lensPicker == nil {
		t.Fatal("the main picker must stay mounted beneath the gallery")
	}
}

func TestLensPickerEnterOnTheAddRowOpensTheGalleryOverThePicker(t *testing.T) {
	model := lensPickerApp(t)

	// One j from the opening cursor is the whole route to it.
	pressLens(t, model, "j")
	if got := lensPickerCursorTitle(t, model); got != lensPickerAddTemplateLabel {
		t.Fatalf("cursor = %q, want the add row", got)
	}
	pressLens(t, model, "enter")

	if model.operationValuePicker == nil {
		t.Fatal("Enter on the add row raised no gallery")
	}
	if got := model.operationValuePicker.provider; got != ValueProviderLensTemplate {
		t.Errorf("gallery provider = %q, want %q", got, ValueProviderLensTemplate)
	}
	if model.lensPicker == nil {
		t.Error("the main picker must stay mounted beneath the gallery")
	}
	if model.contextActionSheet != nil {
		t.Error("the add row opened a management sheet it names no target for")
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("opening the gallery wrote %d lenses", got)
	}
}

func TestLensPickerAddTemplateCancelReturnsToTheMainPicker(t *testing.T) {
	model := lensPickerApp(t)
	openAddTemplateGalleryFromPicker(t, model)

	pressLens(t, model, "esc")

	if model.operationValuePicker != nil {
		t.Error("Esc left the gallery mounted")
	}
	if model.lensPicker == nil {
		t.Fatal("canceling the gallery closed the picker beneath it")
	}
	if got := lensPickerCursorTitle(t, model); got != lensPickerAddTemplateLabel {
		t.Errorf("cursor = %q, want the row the gallery was opened from", got)
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("canceling wrote %d lenses", got)
	}
	if model.crossOutliner.hasActiveLens() {
		t.Error("canceling the gallery applied a lens")
	}
	// The main picker is live again, so the same row still reaches the gallery.
	pressLens(t, model, "enter")
	if model.operationValuePicker == nil {
		t.Fatal("reopening the gallery from the retained picker failed")
	}
}

func TestLensPickerAddTemplateClosesEveryOverlayAndLeavesTheAddedLensActive(t *testing.T) {
	model := lensPickerApp(t)
	template := addableTemplate(t, model)
	openAddTemplateGalleryFromPicker(t, model)
	selectGalleryRow(t, model, template.ID)

	pressLens(t, model, "enter")

	if model.lensPicker != nil || model.contextActionSheet != nil ||
		model.operationValuePicker != nil || model.operationTextPrompt != nil {
		t.Error("adding a built-in view left a lens overlay mounted")
	}
	if model.Overlays().Active() {
		t.Error("adding a built-in view left the overlay manager active")
	}
	saved := lensNamed(t, model, template.Name)
	if saved.ID != template.ID {
		t.Errorf("the added record has ID %s, want the catalog's %s", saved.ID, template.ID)
	}
	if !model.crossOutliner.hasActiveLens() ||
		model.crossOutliner.activeLens.lensID != template.ID {
		t.Error("the added built-in view is not the active lens")
	}
}

func TestLensPickerAddTemplateFailureStaysOnThePickerWithTheReason(t *testing.T) {
	model := lensPickerApp(t)
	template := addableTemplate(t, model)
	openAddTemplateGalleryFromPicker(t, model)
	selectGalleryRow(t, model, template.ID)
	// The gallery drew the row available; the machine changes underneath it, so
	// only the commit-time recheck can refuse. That is the whole point of checking
	// twice.
	stranger := template.Clone()
	stranger.ID = lensTemplateForeignID
	setMachineLenses(t, model, append(machineLenses(t, model).Clone(), stranger))

	pressLens(t, model, "enter")

	if model.operationValuePicker != nil {
		t.Error("a refused add left the gallery mounted")
	}
	if model.lensPicker == nil {
		t.Fatal("a refused add closed the main picker")
	}
	if notice := model.lensPicker.Notice(); !strings.Contains(notice, "already exists") {
		t.Errorf("picker notice = %q, want the duplicate-name reason", notice)
	}
	if got := lensNamed(t, model, template.Name); got.ID != lensTemplateForeignID {
		t.Error("a refused add wrote the template record anyway")
	}
	if model.crossOutliner.hasActiveLens() {
		t.Error("a refused add applied a lens")
	}
	// The reason is retained on the surface that raised it, so the same row is
	// still one Enter away from a corrected retry.
	pressLens(t, model, "enter")
	if model.operationValuePicker == nil {
		t.Fatal("retrying the add raised no gallery")
	}
}

func TestLensPickerSpaceOnTheAddRowExplainsItself(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "j")

	pressLens(t, model, "space")

	if model.contextActionSheet != nil {
		t.Error("the add row named a management target it does not have")
	}
	if model.lensPicker == nil {
		t.Fatal("Space on the add row closed the picker")
	}
	if got := model.lensPicker.Notice(); got != lensPickerAddTemplateSpaceHint {
		t.Errorf("add-row notice = %q, want %q", got, lensPickerAddTemplateSpaceHint)
	}
	// The Save row keeps its own wording rather than inheriting the neighbour's.
	pressLens(t, model, "k", "space")
	if got := model.lensPicker.Notice(); got != lensPickerCreateSpaceNotice {
		t.Errorf("save-row notice = %q, want %q", got, lensPickerCreateSpaceNotice)
	}
}

func TestLensPickerGuidanceNamesTheActUnderTheCursor(t *testing.T) {
	model := lensPickerApp(t)
	footerOf := func() string {
		t.Helper()
		guidance := model.lensPicker.guidanceSpec()
		return ansi.Strip(renderGuidanceText(guidance, 200))
	}

	if got := footerOf(); !strings.Contains(got, lensPickerCreateEnterLabel) {
		t.Errorf("save-row guidance = %q, want %q", got, lensPickerCreateEnterLabel)
	}
	pressLens(t, model, "j")
	if got := footerOf(); !strings.Contains(got, lensPickerAddTemplateEnterLabel) {
		t.Errorf("add-row guidance = %q, want %q", got, lensPickerAddTemplateEnterLabel)
	}
	pressLens(t, model, "j")
	got := footerOf()
	if !strings.Contains(got, "apply") {
		t.Errorf("lens-row guidance = %q, want apply", got)
	}
	if strings.Contains(got, lensPickerAddTemplateEnterLabel) {
		t.Errorf("lens-row guidance = %q, still names the add row's act", got)
	}
	// Space is a management key only on a saved lens, so it is advertised there
	// and nowhere else.
	if !strings.Contains(got, "manage") {
		t.Errorf("lens-row guidance = %q, want the management hint", got)
	}
	pressLens(t, model, "k", "k")
	if got := footerOf(); strings.Contains(got, "manage") {
		t.Errorf("save-row guidance = %q, advertises a sheet it cannot open", got)
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
	want := []string{
		lensPickerCreateLabel, lensPickerAddTemplateLabel,
		"Blocked", "High Priority", "Stale",
	}
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
	if err := lensNamedErr(model, "Waiting"); err != nil {
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
	if err := lensNamedErr(model, "Waiting"); err != nil {
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
	want := []string{
		lensPickerCreateLabel, lensPickerAddTemplateLabel,
		"High Priority", "Waiting",
	}
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

// --- semantic color ---------------------------------------------------------

// lensPickerItemNamed returns one built row by its primary text, which is how a
// color assertion names its target without depending on the row's position.
func lensPickerItemNamed(t *testing.T, model *AppModel, name string) ListItem {
	t.Helper()
	if model.lensPicker == nil {
		t.Fatal("no lens picker is mounted")
	}
	for _, item := range model.lensPicker.surface.Items() {
		if item.PrimaryText == name {
			return item
		}
	}
	t.Fatalf("no picker row named %q in %v", name, lensPickerRowTitles(t, model))
	return ListItem{}
}

// assertLensRowStyle holds one row to the style its own definition resolves to,
// which is the whole contract: the picker never names a color, it asks.
func assertLensRowStyle(t *testing.T, item ListItem, lens config.NamedLens) {
	t.Helper()
	want, ok := namedLensPrimaryStyle(lens)
	if !ok {
		if item.PrimaryStyle != nil {
			t.Fatalf("row %q wears %v for a neutral definition",
				item.PrimaryText, item.PrimaryStyle.GetForeground())
		}
		return
	}
	if item.PrimaryStyle == nil {
		t.Fatalf("row %q carries no style, want %v", item.PrimaryText, want.GetForeground())
	}
	if got := item.PrimaryStyle.GetForeground(); got != want.GetForeground() {
		t.Fatalf("row %q foreground = %v, want %v", item.PrimaryText, got, want.GetForeground())
	}
}

func TestLensPickerColorsSavedRowsAndLeavesActionRowsNeutral(t *testing.T) {
	model := lensPickerApp(t)

	for _, name := range []string{"High Priority", "Stale", "Waiting"} {
		item := lensPickerItemNamed(t, model, name)
		assertLensRowStyle(t, item, lensNamed(t, model, name))
		if item.PrimaryStyle == nil {
			t.Errorf("the %q fixture resolves no accent, so this test proves nothing", name)
		}
		// The accent rides PrimaryStyle alone: the searched, sorted, and truncated
		// text stays the plain name.
		if item.PrimaryText != ansi.Strip(item.PrimaryText) {
			t.Errorf("row %q embeds styling in its primary text: %q", name, item.PrimaryText)
		}
	}
	for _, item := range model.lensPicker.surface.Items() {
		if item.Data != nil {
			continue
		}
		if item.PrimaryStyle != nil {
			t.Errorf("action row %q wears %v", item.PrimaryText, item.PrimaryStyle.GetForeground())
		}
	}
}

// A query change rebuilds every row, so the filtered population must keep the
// color the unfiltered one had, and the ordering must not move.
func TestLensPickerSearchKeepsSavedRowColorAndOrder(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "/")
	typeLens(t, model, "stal")

	want := []string{lensPickerCreateLabel, lensPickerAddTemplateLabel, "Stale"}
	if got := lensPickerRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("rows under a query = %v, want %v", got, want)
	}
	stale := lensPickerItemNamed(t, model, "Stale")
	assertLensRowStyle(t, stale, lensNamed(t, model, "Stale"))
	if stale.PrimaryStyle == nil {
		t.Error("a filtered row lost the accent its definition resolves")
	}
}

func TestLensPickerColorSurvivesRenameAndFollowsReplacement(t *testing.T) {
	model := lensPickerApp(t)
	before := selectLensRow(t, model, "Waiting")
	beforeStyle, ok := namedLensPrimaryStyle(before)
	if !ok {
		t.Fatal("the Waiting fixture resolves no accent")
	}

	pressLens(t, model, "space")
	pressLens(t, model, "enter") // Rename
	for range len("Waiting") {
		pressLens(t, model, "backspace")
	}
	typeLens(t, model, "Blocked")
	pressLens(t, model, "enter")

	// A rename changes no facet, so the same definition must still resolve to the
	// same accent under its new name.
	renamed := lensPickerItemNamed(t, model, "Blocked")
	assertLensRowStyle(t, renamed, lensNamed(t, model, "Blocked"))
	if renamed.PrimaryStyle == nil ||
		renamed.PrimaryStyle.GetForeground() != beforeStyle.GetForeground() {
		t.Fatalf("rename changed the accent to %v, want %v", renamed.PrimaryStyle, beforeStyle.GetForeground())
	}

	// Replacing under the same UUID writes a new definition, which is exactly what
	// the row must recompute from.
	model.crossOutliner.filter.Pinned = true
	pressLens(t, model, "space")
	pressLens(t, model, "j")
	pressLens(t, model, "enter")
	pressLens(t, model, "y")

	replaced := lensNamed(t, model, "Blocked")
	if replaced.ID != before.ID {
		t.Fatal("the replace minted a new identity")
	}
	if got, was := namedLensSemanticRole(replaced), namedLensSemanticRole(before); got == was {
		t.Fatalf("the replacement kept role %s, so recomputation is unobservable", roleText(got))
	}
	assertLensRowStyle(t, lensPickerItemNamed(t, model, "Blocked"), replaced)
}

// Color is read from the definition, never from the record's origin: a built-in
// identity with nothing semantic left in it goes neutral, and a user-created
// UUID carrying a recognized facet is colored like any other.
func TestLensPickerColorFollowsTheDefinitionNotTheBuiltinOrigin(t *testing.T) {
	model := lensPickerApp(t)
	builtinID := lensNamed(t, model, "Waiting").ID

	neutralBuiltin := config.NamedLens{
		ID:   builtinID,
		Name: "Everything",
		Mode: config.LensModeAll,
		Sort: config.LensSort{Field: config.LensSortDefault},
	}
	custom := config.NamedLens{
		ID:    "22222222-2222-4222-8222-222222222222",
		Name:  "Blocked Work",
		Mode:  config.LensModeAll,
		Tasks: config.LensTaskFilters{States: []string{string(models.StateWaiting)}, StatesSet: true},
		Sort:  config.LensSort{Field: config.LensSortDefault},
	}
	if err := model.lensPicker.Refresh(NamedLensPickerSnapshot{
		Lenses: config.NamedLensList{neutralBuiltin, custom},
	}); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if got := lensPickerItemNamed(t, model, "Everything").PrimaryStyle; got != nil {
		t.Errorf("a neutral built-in identity wears %v", got.GetForeground())
	}
	colored := lensPickerItemNamed(t, model, "Blocked Work")
	assertLensRowStyle(t, colored, custom)
	if colored.PrimaryStyle == nil {
		t.Error("a custom lens with an explicit Waiting state received no accent")
	}
}

func TestLensPickerRenderColorsOrdinaryRowsAndYieldsUnderTheCursor(t *testing.T) {
	model := lensPickerApp(t)
	waiting := lensNamed(t, model, "Waiting")
	style, ok := namedLensPrimaryStyle(waiting)
	if !ok {
		t.Fatal("the Waiting fixture resolves no accent")
	}
	if _, err := model.crossOutliner.ApplyNamedLens(waiting); err != nil {
		t.Fatalf("apply lens: %v", err)
	}
	if err := model.lensPicker.Refresh(model.namedLensPickerSnapshot()); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// The cursor opens on the save row, so the active lens is an ordinary row:
	// colored, and still carrying its active marker.
	rendered := model.lensPicker.render(80)
	if !strings.Contains(rendered, style.Render("Waiting")) {
		t.Errorf("an ordinary saved row is not colored:\n%q", rendered)
	}
	if !strings.Contains(ansi.Strip(rendered), "● Waiting") {
		t.Errorf("the active marker no longer precedes the colored name:\n%s", ansi.Strip(rendered))
	}

	// Under the cursor SelectedLine owns the whole line, so the accent must be
	// gone while the text stays exactly where it was.
	if !model.lensPicker.SelectLens(waiting.ID) {
		t.Fatal("the active lens is not listed")
	}
	cursored := model.lensPicker.render(80)
	if strings.Contains(cursored, style.Render("Waiting")) {
		t.Errorf("the cursor row kept its accent:\n%q", cursored)
	}
	if !strings.Contains(ansi.Strip(cursored), "● Waiting") {
		t.Errorf("the cursor row lost the name or its marker:\n%s", ansi.Strip(cursored))
	}
}

// --- rendering --------------------------------------------------------------

func TestLensPickerRendersEmptyNoMatchAndOversizedRows(t *testing.T) {
	model := lensPickerApp(t)

	long := strings.Repeat("Quarterly Planning ", 12)
	oversized := config.NamedLens{
		ID:    "11111111-1111-4111-8111-111111111111",
		Name:  long,
		Mode:  config.LensModeAll,
		Tasks: config.LensTaskFilters{Stale: true, Deferred: true, Pinned: true},
		Sort:  config.LensSort{Field: config.LensSortDefault},
	}
	if _, ok := namedLensPrimaryStyle(oversized); !ok {
		t.Fatal("the oversized fixture is neutral, so it cannot test a colored truncation")
	}
	snapshot := NamedLensPickerSnapshot{
		Lenses:         config.NamedLensList{oversized},
		CurrentSummary: strings.Repeat("summary segment · ", 20),
	}
	if err := model.lensPicker.Refresh(snapshot); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	// The row really is colored while it is being truncated, so the width
	// assertions below are about a styled row rather than a plain one.
	if got := lensPickerItemNamed(t, model, long).PrimaryStyle; got == nil {
		t.Fatal("the oversized row carries no accent, so its truncation is untested")
	}
	for _, width := range []int{minTerminalWidth, 80, 200} {
		rendered := ansi.Strip(model.lensPicker.render(width))
		assertNoLensPickerOverflow(t, rendered, width)
		// An accent is styling, never columns: the name survives truncation to the
		// same visible text a neutral row would have shown.
		if !strings.Contains(rendered, "Quarterly") {
			t.Errorf("the colored row lost its name at width %d:\n%s", width, rendered)
		}
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
	if !strings.Contains(empty, "Add built-in view") {
		t.Errorf("empty render dropped the add row:\n%s", empty)
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

// lensNamedErr reports whether one saved lens exists by name. Only presence is
// ever asserted, so the record itself is deliberately not returned.
func lensNamedErr(model *AppModel, name string) error {
	machine := model.runtime.Machine()
	if machine == nil {
		return errors.New("machine-local state is unavailable")
	}
	for _, lens := range machine.NamedLenses {
		if lens.Name == name {
			return nil
		}
	}
	return fmt.Errorf("no saved lens named %q", name)
}
