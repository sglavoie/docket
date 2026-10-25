package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/lensops"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/lens"
	"github.com/sglavoie/docket/internal/tui/operation"
)

func TestLensPickerCreateClosesEveryOverlayAndLeavesTheNewLensActive(t *testing.T) {
	model := lensPickerApp(t)

	pressLens(t, model, "enter")
	if model.operations.TextPrompt == nil {
		t.Fatal("Enter on the save row raised no name prompt")
	}
	typeLens(t, model, "Focus")
	pressLens(t, model, "enter")

	if model.lenses.Picker() != nil || model.operations.ActionSheet != nil ||
		model.operations.TextPrompt != nil || model.operations.ValuePicker != nil {
		t.Error("creating a lens left a lens overlay mounted")
	}
	if model.Overlays().Active() {
		t.Error("creating a lens left the overlay manager active")
	}
	saved := lensNamed(t, model, "Focus")
	if !model.crossOutliner.HasActiveLens() ||
		model.crossOutliner.Lens.Active.LensID != saved.ID {
		t.Error("the new lens is not the active one")
	}
}

func TestLensPickerCreateCancelReturnsToThePicker(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "enter")
	pressLens(t, model, "esc")

	if model.operations.TextPrompt != nil {
		t.Error("Esc left the name prompt mounted")
	}
	if model.lenses.Picker() == nil {
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

	if model.lenses.Picker() == nil {
		t.Fatal("a refused save closed the picker")
	}
	if notice := model.lenses.Picker().Notice(); !strings.Contains(notice, "already exists") {
		t.Errorf("picker notice = %q, want the duplicate-name reason", notice)
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("a refused save wrote %d lenses", got)
	}
	// The reason is retained on the surface that raised it, so the same row is
	// still one Enter away from a corrected retry.
	pressLens(t, model, "enter")
	if model.operations.TextPrompt == nil {
		t.Fatal("retrying the save raised no name prompt")
	}
}

// --- add built-in view ------------------------------------------------------

// selectGalleryRow parks the gallery cursor on one template by its stable ID, so
// a test names its target without depending on the alphabetical position it
// happens to occupy.
func selectGalleryRow(t *testing.T, model *AppModel, id string) {
	t.Helper()
	if model.operations.ValuePicker == nil {
		t.Fatal("no gallery is mounted")
	}
	if !model.operations.ValuePicker.Surface().SetCursorToID(lensops.TemplateItemID(id)) {
		t.Fatalf("the gallery has no row for template %s", id)
	}
}

// openAddTemplateGalleryFromPicker walks the real nesting: the main picker's
// second action row raises the value step, and the gallery opens above it.
func openAddTemplateGalleryFromPicker(t *testing.T, model *AppModel) {
	t.Helper()
	if !model.lenses.Picker().Surface.SetCursorToID(lens.LensPickerAddTemplateItemID) {
		t.Fatal("the picker has no Add built-in view row")
	}
	pressLens(t, model, "enter")
	if model.operations.ValuePicker == nil {
		t.Fatal("Enter on the add row raised no gallery")
	}
	if model.lenses.Picker() == nil {
		t.Fatal("the main picker must stay mounted beneath the gallery")
	}
}

func TestLensPickerEnterOnTheAddRowOpensTheGalleryOverThePicker(t *testing.T) {
	model := lensPickerApp(t)

	// One j from the opening cursor is the whole route to it.
	pressLens(t, model, "j")
	if got := lensPickerCursorTitle(t, model); got != lens.LensPickerAddTemplateLabel {
		t.Fatalf("cursor = %q, want the add row", got)
	}
	pressLens(t, model, "enter")

	if model.operations.ValuePicker == nil {
		t.Fatal("Enter on the add row raised no gallery")
	}
	if got := model.operations.ValuePicker.Provider(); got != operation.ValueProviderLensTemplate {
		t.Errorf("gallery provider = %q, want %q", got, operation.ValueProviderLensTemplate)
	}
	if model.lenses.Picker() == nil {
		t.Error("the main picker must stay mounted beneath the gallery")
	}
	if model.operations.ActionSheet != nil {
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

	if model.operations.ValuePicker != nil {
		t.Error("Esc left the gallery mounted")
	}
	if model.lenses.Picker() == nil {
		t.Fatal("canceling the gallery closed the picker beneath it")
	}
	if got := lensPickerCursorTitle(t, model); got != lens.LensPickerAddTemplateLabel {
		t.Errorf("cursor = %q, want the row the gallery was opened from", got)
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Errorf("canceling wrote %d lenses", got)
	}
	if model.crossOutliner.HasActiveLens() {
		t.Error("canceling the gallery applied a lens")
	}
	// The main picker is live again, so the same row still reaches the gallery.
	pressLens(t, model, "enter")
	if model.operations.ValuePicker == nil {
		t.Fatal("reopening the gallery from the retained picker failed")
	}
}

func TestLensPickerAddTemplateClosesEveryOverlayAndLeavesTheAddedLensActive(t *testing.T) {
	model := lensPickerApp(t)
	template := addableTemplate(t, model)
	openAddTemplateGalleryFromPicker(t, model)
	selectGalleryRow(t, model, template.ID)

	pressLens(t, model, "enter")

	if model.lenses.Picker() != nil || model.operations.ActionSheet != nil ||
		model.operations.ValuePicker != nil || model.operations.TextPrompt != nil {
		t.Error("adding a built-in view left a lens overlay mounted")
	}
	if model.Overlays().Active() {
		t.Error("adding a built-in view left the overlay manager active")
	}
	saved := lensNamed(t, model, template.Name)
	if saved.ID != template.ID {
		t.Errorf("the added record has ID %s, want the catalog's %s", saved.ID, template.ID)
	}
	if !model.crossOutliner.HasActiveLens() ||
		model.crossOutliner.Lens.Active.LensID != template.ID {
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

	if model.operations.ValuePicker != nil {
		t.Error("a refused add left the gallery mounted")
	}
	if model.lenses.Picker() == nil {
		t.Fatal("a refused add closed the main picker")
	}
	if notice := model.lenses.Picker().Notice(); !strings.Contains(notice, "already exists") {
		t.Errorf("picker notice = %q, want the duplicate-name reason", notice)
	}
	if got := lensNamed(t, model, template.Name); got.ID != lensTemplateForeignID {
		t.Error("a refused add wrote the template record anyway")
	}
	if model.crossOutliner.HasActiveLens() {
		t.Error("a refused add applied a lens")
	}
	// The reason is retained on the surface that raised it, so the same row is
	// still one Enter away from a corrected retry.
	pressLens(t, model, "enter")
	if model.operations.ValuePicker == nil {
		t.Fatal("retrying the add raised no gallery")
	}
}

func TestLensPickerSpaceOnTheAddRowExplainsItself(t *testing.T) {
	model := lensPickerApp(t)
	pressLens(t, model, "j")

	pressLens(t, model, "space")

	if model.operations.ActionSheet != nil {
		t.Error("the add row named a management target it does not have")
	}
	if model.lenses.Picker() == nil {
		t.Fatal("Space on the add row closed the picker")
	}
	if got := model.lenses.Picker().Notice(); got != lens.LensPickerAddTemplateSpaceHint {
		t.Errorf("add-row notice = %q, want %q", got, lens.LensPickerAddTemplateSpaceHint)
	}
	// The Save row keeps its own wording rather than inheriting the neighbour's.
	pressLens(t, model, "k", "space")
	if got := model.lenses.Picker().Notice(); got != lens.LensPickerCreateSpaceNotice {
		t.Errorf("save-row notice = %q, want %q", got, lens.LensPickerCreateSpaceNotice)
	}
}
