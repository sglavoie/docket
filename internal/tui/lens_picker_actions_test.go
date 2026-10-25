package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
)

func TestLensPickerGuidanceNamesTheActUnderTheCursor(t *testing.T) {
	model := lensPickerApp(t)
	footerOf := func() string {
		t.Helper()
		guidance := model.lenses.Picker().GuidanceSpec()
		return ansi.Strip(chrome.RenderGuidanceText(guidance, 200))
	}

	if got := footerOf(); !strings.Contains(got, lens.LensPickerCreateEnterLabel) {
		t.Errorf("save-row guidance = %q, want %q", got, lens.LensPickerCreateEnterLabel)
	}
	pressLens(t, model, "j")
	if got := footerOf(); !strings.Contains(got, lens.LensPickerAddTemplateEnterLabel) {
		t.Errorf("add-row guidance = %q, want %q", got, lens.LensPickerAddTemplateEnterLabel)
	}
	pressLens(t, model, "j")
	got := footerOf()
	if !strings.Contains(got, "apply") {
		t.Errorf("lens-row guidance = %q, want apply", got)
	}
	if strings.Contains(got, lens.LensPickerAddTemplateEnterLabel) {
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

	if model.operations.ActionSheet == nil {
		t.Fatal("Space raised no action sheet")
	}
	if model.lenses.Picker() == nil {
		t.Error("the picker must stay mounted beneath the action sheet")
	}
	want := []string{"Rename Lens", "Replace from Current View", "Duplicate Lens", "Delete Lens"}
	if got := sheetRowTitles(t, model); !slices.Equal(got, want) {
		t.Fatalf("sheet rows = %v, want %v", got, want)
	}
}

func TestLensPickerSpaceOnTheSaveRowRaisesNoSheet(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "space")

	if model.operations.ActionSheet != nil {
		t.Error("the save row named a management target it does not have")
	}
	if model.lenses.Picker() == nil {
		t.Fatal("Space on the save row closed the picker")
	}
}

func TestLensPickerSheetEscapeReturnsToThePicker(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Stale")
	pressLens(t, model, "space")

	pressLens(t, model, "esc")

	if model.operations.ActionSheet != nil {
		t.Error("Esc left the action sheet mounted")
	}
	if model.lenses.Picker() == nil {
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

	if model.operations.TextPrompt == nil {
		t.Fatal("Rename raised no name prompt")
	}
	// The prompt prefills the current name, so clearing it is what a real rename
	// types over.
	for range len("Waiting") {
		pressLens(t, model, "backspace")
	}
	typeLens(t, model, "Blocked")
	pressLens(t, model, "enter")

	if model.operations.ActionSheet != nil || model.operations.TextPrompt != nil {
		t.Error("a completed rename left a nested overlay mounted")
	}
	if model.lenses.Picker() == nil {
		t.Fatal("a completed rename closed the picker")
	}
	want := []string{
		lens.LensPickerCreateLabel, lens.LensPickerAddTemplateLabel,
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

	if model.operations.TextPrompt != nil {
		t.Error("Esc left the rename prompt mounted")
	}
	if model.operations.ActionSheet == nil {
		t.Fatal("canceling the rename must return to the action sheet")
	}
	if model.lenses.Picker() == nil {
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

	if model.operations.ActionSheet == nil {
		t.Fatal("a refused rename closed the action sheet")
	}
	if notice := model.operations.ActionSheet.Notice; !strings.Contains(notice, "already exists") {
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

	if !model.operations.Confirm.IsActive() {
		t.Fatal("Replace raised no confirmation")
	}

	pressLens(t, model, "esc")

	if model.operations.Confirm.IsActive() {
		t.Error("Esc left the confirmation active")
	}
	if model.operations.ActionSheet == nil {
		t.Fatal("canceling the confirmation must return to the action sheet")
	}
	if model.lenses.Picker() == nil {
		t.Error("canceling the confirmation closed the picker")
	}
	if saved := lensNamed(t, model, "Waiting"); saved.Query != "$waiting" {
		t.Errorf("a canceled replace overwrote the saved definition: %q", saved.Query)
	}
}

func TestLensReplaceConfirmedReturnsToTheRefreshedPicker(t *testing.T) {
	model := lensPickerApp(t)
	target := selectLensRow(t, model, "Waiting")
	// Give the live view a facet the saved definition does not carry, so the
	// replacement is observable.
	model.crossOutliner.Filter.Pinned = true
	adoptFilterCarriers(model.crossOutliner)
	pressLens(t, model, "space")
	pressLens(t, model, "j")
	pressLens(t, model, "enter")
	pressLens(t, model, "y")

	if model.operations.ActionSheet != nil || model.operations.Confirm.IsActive() {
		t.Error("a completed replace left a nested overlay mounted")
	}
	if model.lenses.Picker() == nil {
		t.Fatal("a completed replace closed the picker")
	}
	if got := lensPickerCursorTitle(t, model); got != "Waiting" {
		t.Errorf("cursor = %q, want the replaced lens", got)
	}
	saved := lensNamed(t, model, "Waiting")
	if saved.ID != target.ID {
		t.Error("the replace minted a new identity")
	}
	if !strings.Contains(saved.Query, "*pin") {
		t.Errorf("the replacement did not capture the live view: %q", saved.Query)
	}
}

// --- delete -----------------------------------------------------------------

func TestLensDeleteReturnsToTheNearestSurvivingRow(t *testing.T) {
	model := lensPickerApp(t)
	selectLensRow(t, model, "Stale")
	pressLens(t, model, "space")
	pressLens(t, model, "j", "j", "j")
	pressLens(t, model, "enter") // Delete lens
	if !model.operations.Confirm.IsActive() {
		t.Fatal("Delete raised no confirmation")
	}
	pressLens(t, model, "y")

	if model.lenses.Picker() == nil {
		t.Fatal("a completed delete closed the picker")
	}
	want := []string{
		lens.LensPickerCreateLabel, lens.LensPickerAddTemplateLabel,
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
	baseMode := model.crossOutliner.PrimaryMode
	waiting := lensNamed(t, model, "Waiting")
	if _, err := model.crossOutliner.ApplyNamedLens(waiting); err != nil {
		t.Fatalf("apply lens: %v", err)
	}
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}
	selectLensRow(t, model, "Waiting")
	pressLens(t, model, "space")
	pressLens(t, model, "j", "j", "j")
	pressLens(t, model, "enter")
	pressLens(t, model, "y")

	if model.lenses.Picker() == nil {
		t.Fatal("deleting the active lens closed the picker")
	}
	if model.crossOutliner.HasActiveLens() {
		t.Error("the deleted lens is still active")
	}
	if got := model.crossOutliner.PrimaryMode; got != baseMode {
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
	pressLens(t, model, "j", "j", "j")
	pressLens(t, model, "enter")
	pressLens(t, model, "n")

	if model.operations.ActionSheet == nil {
		t.Fatal("canceling the confirmation must return to the action sheet")
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("a canceled delete left %d lenses", got)
	}
	if model.operations.TakeRequest().ID != "" {
		t.Error("a canceled confirmation left a pending request")
	}
}

// --- semantic color ---------------------------------------------------------
