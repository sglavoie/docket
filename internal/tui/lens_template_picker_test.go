package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// lensTemplateForeignID is a UUID outside the frozen `6f1a0c4e-…` catalog
// prefix, so a record built with it can share a template's name without ever
// sharing its stable identity.
const lensTemplateForeignID = "11111111-2222-4333-8444-555555555501"

// openLensTemplateGallery opens the gallery through the real path: the executor
// asks for a value, and the shell's value step builds the overlay. Nothing here
// names a task, which is the point — the catalog is global.
func openLensTemplateGallery(t *testing.T, model *AppModel) *OperationValuePickerModel {
	t.Helper()
	result := executeLens(model, operationLensAddTemplate, operation.EntityIdentity{}, "", false)
	if result.ValueRequired == nil {
		t.Fatalf("add template did not ask for a value: %#v", result)
	}
	if cmd := model.openOperationValueStep(*result.ValueRequired); cmd != nil {
		cmd()
	}
	if model.err != nil {
		t.Fatalf("opening the gallery failed: %v", model.err)
	}
	if model.operationValuePicker == nil {
		t.Fatal("the gallery did not open")
	}
	return model.operationValuePicker
}

// setMachineLenses replaces the machine-local saved list in memory, which is the
// same seam a commit publishes through.
func setMachineLenses(t *testing.T, model *AppModel, lenses config.NamedLensList) {
	t.Helper()
	machine := cloneMachineState(model.runtime.Machine())
	machine.NamedLenses = lenses
	if err := machine.Validate(); err != nil {
		t.Fatalf("the test machine state is invalid: %v", err)
	}
	model.runtime.SetMachineInMemory(machine)
}

func lensTemplateByName(t *testing.T, name string) config.NamedLens {
	t.Helper()
	for _, template := range config.BuiltinNamedLensTemplates() {
		if template.Name == name {
			return template
		}
	}
	t.Fatalf("no built-in template is named %q", name)
	return config.NamedLens{}
}

// galleryRow finds one template's row by its stable ID rather than by position,
// so an ordering change fails the ordering test alone.
func galleryRow(t *testing.T, picker *OperationValuePickerModel, id string) ListItem {
	t.Helper()
	for _, item := range picker.surface.Items() {
		if item.ID == lensTemplateItemID(id) {
			return item
		}
	}
	t.Fatalf("the gallery has no row for template %s", id)
	return ListItem{}
}

func typeIntoGallery(picker *OperationValuePickerModel, query string) {
	picker.handleKey(tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range query {
		picker.handleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func galleryMatchNames(picker *OperationValuePickerModel) []string {
	names := make([]string, 0, len(picker.surface.Matches()))
	for _, match := range picker.surface.Matches() {
		names = append(names, match.Item.PrimaryText)
	}
	return names
}

func TestLensTemplateGalleryOpensWithoutAnyTaskContext(t *testing.T) {
	model := lensOperationApp(t)
	// The Outliner starts with no cursor entity at all, so a provider that
	// resolved task refs would fail here instead of rendering the catalog.
	picker := openLensTemplateGallery(t, model)

	if picker.provider != operation.ValueProviderLensTemplate {
		t.Fatalf("gallery provider = %q, want %q", picker.provider, operation.ValueProviderLensTemplate)
	}
	if picker.multi {
		t.Fatal("the gallery is a single-select picker")
	}
	if picker.title != "Add Built-in View" {
		t.Fatalf("gallery title = %q, want Add Built-in View", picker.title)
	}
	if picker.subtitle != "Choose a lens template" {
		t.Fatalf("gallery subtitle = %q, want Choose a lens template", picker.subtitle)
	}
	if picker.provider.TextEntry() || picker.provider.TagEntry() || picker.provider.DateEntry() {
		t.Fatal("the gallery provider must be a plain list, not a text, tag, or date entry")
	}
}

func TestLensTemplateGalleryListsEveryTemplateAlphabetically(t *testing.T) {
	model := lensOperationApp(t)
	picker := openLensTemplateGallery(t, model)

	items := picker.surface.Items()
	want := []string{
		"Active Work",
		"Cleanup 90d",
		"Deferred",
		"High Priority",
		"Overdue",
		"Parked Work",
		"Review Queue",
		"Stale",
		"Waiting",
	}
	if len(items) != len(want) {
		t.Fatalf("gallery listed %d templates, want %d", len(items), len(want))
	}
	for index, name := range want {
		if items[index].PrimaryText != name {
			t.Errorf("gallery row %d = %q, want %q", index, items[index].PrimaryText, name)
		}
	}
	for _, template := range config.BuiltinNamedLensTemplates() {
		row := galleryRow(t, picker, template.ID)
		summary := namedLensSummary(template)
		if row.SecondaryText != summary {
			t.Errorf("%s secondary = %q, want %q", template.Name, row.SecondaryText, summary)
		}
		if payload, ok := row.Data.(string); !ok || payload != template.ID {
			t.Errorf("%s payload = %#v, want the stable ID %s", template.Name, row.Data, template.ID)
		}
		if !containsString(row.Aliases, summary) {
			t.Errorf("%s aliases = %v, want the summary among them", template.Name, row.Aliases)
		}
		if !containsString(row.Aliases, template.Name) {
			t.Errorf("%s aliases = %v, want the name among them", template.Name, row.Aliases)
		}
	}
}

func TestLensTemplateGalleryDisablesTheSeededStartersAndOffersTheRest(t *testing.T) {
	model := lensOperationApp(t)
	seeded := machineLenses(t, model)
	if len(seeded) != 3 {
		t.Fatalf("the test workspace seeded %d lenses, want the three starters", len(seeded))
	}
	picker := openLensTemplateGallery(t, model)

	// Every row stays visible: the gallery says why a template is unavailable
	// rather than hiding the fact that it exists.
	if len(picker.surface.Items()) != 9 {
		t.Fatalf("the gallery hid rows: %d visible, want 9", len(picker.surface.Items()))
	}
	for _, lens := range seeded {
		row := galleryRow(t, picker, lens.ID)
		want := fmt.Sprintf("Already added as %q", lens.Name)
		if row.DisabledReason != want {
			t.Errorf("%s disabled reason = %q, want %q", lens.Name, row.DisabledReason, want)
		}
	}
	added := 0
	for _, template := range config.BuiltinNamedLensTemplates() {
		if galleryRow(t, picker, template.ID).Enabled() {
			added++
		}
	}
	if added != 6 {
		t.Fatalf("%d templates are addable beside the three starters, want 6", added)
	}
}

func TestLensTemplateGalleryOffersEveryTemplateOnAnEmptyMachine(t *testing.T) {
	model := lensOperationApp(t)
	setMachineLenses(t, model, config.NamedLensList{})
	picker := openLensTemplateGallery(t, model)

	for _, template := range config.BuiltinNamedLensTemplates() {
		row := galleryRow(t, picker, template.ID)
		if !row.Enabled() {
			t.Errorf("%s is unavailable on an empty machine: %q", template.Name, row.DisabledReason)
		}
	}
}

func TestLensTemplateGalleryKeepsARenamedTemplateUnavailableUnderItsCurrentName(t *testing.T) {
	model := lensOperationApp(t)
	waiting := lensTemplateByName(t, "Waiting")
	renamed := waiting.Clone()
	renamed.Name = "Blocked On Someone"
	setMachineLenses(t, model, config.NamedLensList{renamed})

	picker := openLensTemplateGallery(t, model)
	row := galleryRow(t, picker, waiting.ID)
	if want := `Already added as "Blocked On Someone"`; row.DisabledReason != want {
		t.Fatalf("renamed template reason = %q, want %q", row.DisabledReason, want)
	}
	// The name it vacated is free again, so nothing else is dragged down with it.
	if other := galleryRow(t, picker, lensTemplateByName(t, "Overdue").ID); !other.Enabled() {
		t.Fatalf("Overdue became unavailable: %q", other.DisabledReason)
	}
}

func TestLensTemplateGalleryKeepsAReplacedTemplateUnavailable(t *testing.T) {
	model := lensOperationApp(t)
	stale := lensTemplateByName(t, "Stale")
	replaced := stale.Clone()
	// A replace overwrites the projection under the same identity, which is
	// exactly what the stable-ID rule is there to keep unavailable.
	replaced.Query = "invoice"
	setMachineLenses(t, model, config.NamedLensList{replaced})

	picker := openLensTemplateGallery(t, model)
	row := galleryRow(t, picker, stale.ID)
	if want := `Already added as "Stale"`; row.DisabledReason != want {
		t.Fatalf("replaced template reason = %q, want %q", row.DisabledReason, want)
	}
}

func TestLensTemplateGalleryReportsANameCollisionUnderAForeignID(t *testing.T) {
	model := lensOperationApp(t)
	overdue := lensTemplateByName(t, "Overdue")
	stranger := overdue.Clone()
	stranger.ID = lensTemplateForeignID
	// Uniqueness is case-insensitive, so a lowercase stranger still blocks the
	// template's name.
	stranger.Name = "overdue"
	setMachineLenses(t, model, config.NamedLensList{stranger})

	picker := openLensTemplateGallery(t, model)
	row := galleryRow(t, picker, overdue.ID)
	if want := `A lens named "Overdue" already exists`; row.DisabledReason != want {
		t.Fatalf("name collision reason = %q, want %q", row.DisabledReason, want)
	}
}

func TestLensTemplateGalleryPrefersTheIDCollisionOverTheNameCollision(t *testing.T) {
	model := lensOperationApp(t)
	deferred := lensTemplateByName(t, "Deferred")
	renamed := deferred.Clone()
	renamed.Name = "Snoozed"
	stranger := deferred.Clone()
	stranger.ID = lensTemplateForeignID
	stranger.Name = deferred.Name
	setMachineLenses(t, model, config.NamedLensList{renamed, stranger})

	picker := openLensTemplateGallery(t, model)
	row := galleryRow(t, picker, deferred.ID)
	if want := `Already added as "Snoozed"`; row.DisabledReason != want {
		t.Fatalf("collision reason = %q, want the stable-ID refusal %q", row.DisabledReason, want)
	}
}

func TestLensTemplateGallerySearchesNamesAndFacetSummaries(t *testing.T) {
	t.Run("by name", func(t *testing.T) {
		model := lensOperationApp(t)
		picker := openLensTemplateGallery(t, model)
		typeIntoGallery(picker, "cleanup")
		if names := galleryMatchNames(picker); len(names) != 1 || names[0] != "Cleanup 90d" {
			t.Fatalf("search for cleanup matched %v, want [Cleanup 90d]", names)
		}
	})

	t.Run("by facet summary", func(t *testing.T) {
		model := lensOperationApp(t)
		picker := openLensTemplateGallery(t, model)
		// "doing" appears only in Active Work's summary, never in a template name.
		typeIntoGallery(picker, "doing")
		names := galleryMatchNames(picker)
		if len(names) != 1 || names[0] != "Active Work" {
			t.Fatalf("search for doing matched %v, want [Active Work]", names)
		}
	})

	t.Run("search does not hide unavailable rows", func(t *testing.T) {
		model := lensOperationApp(t)
		picker := openLensTemplateGallery(t, model)
		typeIntoGallery(picker, "waiting")
		names := galleryMatchNames(picker)
		if len(names) != 1 || names[0] != "Waiting" {
			t.Fatalf("search for waiting matched %v, want the disabled [Waiting]", names)
		}
	})
}

func TestLensTemplateGalleryRefusesADisabledRowAndStaysOpen(t *testing.T) {
	model := lensOperationApp(t)
	waiting := lensTemplateByName(t, "Waiting")
	picker := openLensTemplateGallery(t, model)
	if !picker.surface.SetCursorToID(lensTemplateItemID(waiting.ID)) {
		t.Fatal("the cursor could not reach the seeded Waiting row")
	}

	_, dismissed, msg := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed {
		t.Fatal("a refused row closed the gallery")
	}
	status, ok := msg.(status.StatusMsg)
	if !ok {
		t.Fatalf("refusal message = %#v, want a StatusMsg", msg)
	}
	if want := `Already added as "Waiting"`; !strings.Contains(string(status), want) {
		t.Fatalf("refusal status = %q, want it to name %q", string(status), want)
	}
	if _, err := picker.selectedValue(); !errors.Is(err, ErrListItemDisabled) {
		t.Fatalf("selectedValue error = %v, want ErrListItemDisabled", err)
	}
}

func TestLensTemplateGallerySelectsTheStableTemplateID(t *testing.T) {
	model := lensOperationApp(t)
	overdue := lensTemplateByName(t, "Overdue")
	picker := openLensTemplateGallery(t, model)
	if !picker.surface.SetCursorToID(lensTemplateItemID(overdue.ID)) {
		t.Fatal("the cursor could not reach the Overdue row")
	}

	_, dismissed, msg := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("selecting a template left the gallery open")
	}
	selected, ok := msg.(operation.OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("selection message = %#v, want OperationValueSelectedMsg", msg)
	}
	if selected.Request.ID != operationLensAddTemplate {
		t.Fatalf("resumed request = %q, want %q", selected.Request.ID, operationLensAddTemplate)
	}
	payload, ok := selected.Value.(string)
	if !ok || payload != overdue.ID {
		t.Fatalf("selected value = %#v, want the stable ID %s", selected.Value, overdue.ID)
	}
}

// The gallery previews the same accent the saved picker will show once a
// template is added, and it does so by asking the definition rather than by
// knowing anything about the catalog. Everything below holds that seam: the
// style is metadata beside the stable payload, an unavailable row keeps the
// metadata but renders muted, and the two surfaces never disagree.
