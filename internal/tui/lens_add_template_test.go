package tui

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// hands back an ID, and the malformed-payload cases need to deliver something
// that is not a string at all. Add built-in view is the one operation that takes
// a value here, so it is named once rather than passed by every caller.
func executeLensValue(model *AppModel, value any) operation.OperationExecutionResult {
	return model.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:       lens.OperationLensAddTemplate,
		Context:  lensOperationContext(operation.EntityIdentity{}),
		Value:    value,
		HasValue: true,
	})
}

// addableTemplate returns the first catalog template the machine does not hold
// yet. It is resolved rather than named so the tests follow the seeded prefix
// wherever the catalog moves it, instead of freezing a second copy of it here.
func addableTemplate(t *testing.T, model *AppModel) config.NamedLens {
	t.Helper()
	machine := model.runtime.Machine()
	for _, template := range config.BuiltinNamedLensTemplates() {
		if machine.NamedLensByID(template.ID) == nil {
			return template
		}
	}
	t.Fatal("every built-in template is already saved; no addable one remains")
	return config.NamedLens{}
}

func TestLensAddTemplateIsAZeroTargetPickedMutation(t *testing.T) {
	model := lensOperationApp(t)
	descriptor, ok := model.operationRegistry.Descriptor(lens.OperationLensAddTemplate)
	if !ok {
		t.Fatal("the add-template operation is not registered")
	}
	if descriptor.ID != "tui.lens.add-template" {
		t.Fatalf("descriptor ID = %q, want the stable tui.lens.add-template", descriptor.ID)
	}
	if descriptor.Label != "Add built-in view" {
		t.Fatalf("descriptor label = %q", descriptor.Label)
	}
	if descriptor.Category != operation.OperationCategoryFilter {
		t.Fatalf("descriptor category = %q, want the filter family", descriptor.Category)
	}
	if descriptor.TargetSource != operation.OperationTargetGlobal ||
		descriptor.TargetCardinality != operation.TargetCardinalityNone {
		t.Fatalf(
			"descriptor targets %q/%q, want a global zero-target operation",
			descriptor.TargetSource,
			descriptor.TargetCardinality,
		)
	}
	if !descriptor.RequiredCapabilities.Has(operation.CapabilityLensPicker) {
		t.Fatal("descriptor does not require the lens picker capability")
	}
	if descriptor.Risk != operation.OperationRiskMutation {
		t.Fatalf("descriptor risk = %q, want mutation", descriptor.Risk)
	}
	if descriptor.ValueProviderID != operation.ValueProviderLensTemplate {
		t.Fatalf("descriptor value provider = %q, want %q", descriptor.ValueProviderID, operation.ValueProviderLensTemplate)
	}
	// The template is picked from a closed catalog, so it must not route to the
	// typed-text prompt the lens name uses.
	if descriptor.ValueProviderID.TextEntry() {
		t.Fatal("the template provider must be picked, not typed")
	}
}

func TestLensAddTemplateAsksForATemplateBeforeItWrites(t *testing.T) {
	model := lensOperationApp(t)
	before := len(machineLenses(t, model))

	result := executeLens(model, lens.OperationLensAddTemplate, operation.EntityIdentity{}, "", false)
	if result.ValueRequired == nil || result.ValueRequired.Provider != operation.ValueProviderLensTemplate {
		t.Fatalf("add template ran without asking for a template: %#v", result.ValueRequired)
	}
	if got := len(machineLenses(t, model)); got != before {
		t.Fatalf("an unanswered template prompt wrote %d records, want %d", got, before)
	}
}

func TestLensAddTemplatePersistsTheCatalogDefinitionAndAppliesIt(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	out.SelectOutlinerMode(chrome.ModeInbox)
	template := addableTemplate(t, model)
	before := len(machineLenses(t, model))

	result := executeLensValue(model, template.ID)
	if result.Err != nil {
		t.Fatalf("add built-in view %q: %v", template.Name, result.Err)
	}
	if result.Status != "Added built-in view: "+template.Name {
		t.Fatalf("status = %q, want the add confirmation", result.Status)
	}
	if got := len(machineLenses(t, model)); got != before+1 {
		t.Fatalf("saved list = %d records, want %d", got, before+1)
	}

	saved := model.runtime.Machine().NamedLensByID(template.ID)
	if saved == nil {
		t.Fatalf("no record saved under the template ID %q", template.ID)
	}
	// The record is the catalog's own definition, not a projection of the current
	// view: every facet must survive the round trip byte for byte.
	if !reflect.DeepEqual(*saved, template) {
		t.Fatalf("saved record = %+v, want the catalog definition %+v", *saved, template)
	}
	if !out.HasActiveLens() || out.Lens.Active.LensID != template.ID {
		t.Fatalf("added template is not the live lens: %v", out.ActiveLensLabel())
	}
	if out.LensModified() {
		t.Fatal("a freshly added built-in view starts unmodified")
	}
	// One Esc still returns to the pre-lens view, so the application kept the
	// baseline rather than stacking a new one.
	out.RestoreLensBaseline()
	if out.HasActiveLens() || out.PrimaryMode != chrome.ModeInbox {
		t.Fatalf("one restore left mode %v with lens=%v", out.PrimaryMode, out.HasActiveLens())
	}
	// Machine-local state rides no workspace transaction, so nothing here may
	// reach the executor's undo ledger.
	if undo := model.operationExecutor.Undo(); undo.Err == nil {
		t.Fatal("expected no undo entry for an added built-in view")
	}
	if redo := model.operationExecutor.Redo(); redo.Err == nil {
		t.Fatal("expected no redo entry for an added built-in view")
	}
}

func TestLensAddTemplateRejectsMalformedAndUnknownPayloads(t *testing.T) {
	model := lensOperationApp(t)
	template := addableTemplate(t, model)
	before := len(machineLenses(t, model))

	for _, testCase := range []struct {
		name  string
		value any
		want  string
	}{
		// A definition captured by the UI is exactly what must not be trusted: only
		// the ID crosses the seam.
		{"a captured definition", template, "must name a template ID"},
		{"a non-string payload", 4, "must name a template ID"},
		{"an unknown template ID", lensTestID(42), "unknown built-in view"},
		{"an empty ID", "", "unknown built-in view"},
		{"a name instead of an ID", template.Name, "unknown built-in view"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result := executeLensValue(model, testCase.value)
			if result.Err == nil || !strings.Contains(result.Err.Error(), testCase.want) {
				t.Fatalf("add with %s = %v, want an error naming %q", testCase.name, result.Err, testCase.want)
			}
			if got := len(machineLenses(t, model)); got != before {
				t.Fatalf("a refused add wrote %d records, want %d", got, before)
			}
			if model.crossOutliner.HasActiveLens() {
				t.Fatal("a refused add applied a lens")
			}
		})
	}
}

// Availability is decided twice: once when the gallery renders and again here,
// against the snapshot the commit actually reads. The stable ID answers first,
// so a template that was renamed after it was added stays unavailable under the
// name it now carries.
func TestLensAddTemplateRefusesATemplateTheLatestSnapshotAlreadyHolds(t *testing.T) {
	model := lensOperationApp(t)
	template := addableTemplate(t, model)

	if result := executeLensValue(model, template.ID); result.Err != nil {
		t.Fatalf("add built-in view: %v", result.Err)
	}
	added := len(machineLenses(t, model))

	result := executeLensValue(model, template.ID)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "already added as") {
		t.Fatalf("re-adding a template = %v, want an already-added refusal", result.Err)
	}

	if renamed := executeLens(
		model, lens.OperationLensRename, operation.LensEntity(template.ID), "Renamed by hand", false,
	); renamed.Err != nil {
		t.Fatalf("rename the added template: %v", renamed.Err)
	}
	result = executeLensValue(model, template.ID)
	if result.Err == nil || !strings.Contains(result.Err.Error(), `already added as "Renamed by hand"`) {
		t.Fatalf("re-adding a renamed template = %v, want a refusal naming its current name", result.Err)
	}
	if got := len(machineLenses(t, model)); got != added {
		t.Fatalf("refused re-adds left %d records, want %d", got, added)
	}

	// With the stable ID freed, the template is addable again — an added record is
	// an ordinary lens and nothing more.
	if deleted := executeLens(
		model, lens.OperationLensDelete, operation.LensEntity(template.ID), "", true,
	); deleted.Err != nil {
		t.Fatalf("delete the added template: %v", deleted.Err)
	}
	if result := executeLensValue(model, template.ID); result.Err != nil {
		t.Fatalf("re-adding a deleted template = %v, want success", result.Err)
	}
}

func TestLensAddTemplateRefusesANameAnotherLensAlreadyAnswersTo(t *testing.T) {
	model := lensOperationApp(t)
	template := addableTemplate(t, model)

	// A user lens minted under its own UUID, wearing the template's name in a
	// different case: uniqueness is case-insensitive, so the template cannot land.
	if created := executeLens(
		model, lens.OperationLensCreate, operation.EntityIdentity{}, strings.ToUpper(template.Name), false,
	); created.Err != nil {
		t.Fatalf("create the colliding lens: %v", created.Err)
	}
	before := len(machineLenses(t, model))

	result := executeLensValue(model, template.ID)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "already exists") {
		t.Fatalf("adding a name-colliding template = %v, want a name-collision refusal", result.Err)
	}
	if got := len(machineLenses(t, model)); got != before {
		t.Fatalf("a refused add wrote %d records, want %d", got, before)
	}
	if model.runtime.Machine().NamedLensByID(template.ID) != nil {
		t.Fatal("a refused add saved the template anyway")
	}
}

// Both collisions can hold at once. The stable ID is the one reported, because
// "already added" is the more useful sentence when the template is in fact
// already there under another name.
func TestLensAddTemplateReportsTheIDCollisionAheadOfTheNameCollision(t *testing.T) {
	model := lensOperationApp(t)
	template := addableTemplate(t, model)

	if result := executeLensValue(model, template.ID); result.Err != nil {
		t.Fatalf("add built-in view: %v", result.Err)
	}
	if renamed := executeLens(
		model, lens.OperationLensRename, operation.LensEntity(template.ID), "Renamed by hand", false,
	); renamed.Err != nil {
		t.Fatalf("rename the added template: %v", renamed.Err)
	}
	// The template's own name is free again, so a user lens can take it.
	if created := executeLens(
		model, lens.OperationLensCreate, operation.EntityIdentity{}, template.Name, false,
	); created.Err != nil {
		t.Fatalf("create the name-colliding lens: %v", created.Err)
	}

	result := executeLensValue(model, template.ID)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "already added as") {
		t.Fatalf("both collisions = %v, want the already-added refusal", result.Err)
	}
}

// The commit is what publishes the record, and the application runs only after
// it returns nil. A commit that fails on disk therefore leaves the machine, the
// cached snapshot, and the current view exactly as they were.
func TestAFailedAddTemplateCommitAppliesNothing(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	dataDir := t.TempDir()
	project := models.Project{ID: "alpha", Name: "alpha", Slug: "alpha"}
	model.runtime = testRuntimeAt(dataDir, testConfigWithProjects(&project))
	template := addableTemplate(t, model)

	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Chmod(home, 0o500); err != nil {
		t.Skipf("cannot make the runtime root read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	before := model.runtime.Machine()
	result := executeLensValue(model, template.ID)
	if result.Err == nil {
		t.Skip("the runtime root stayed writable; the failure path is untested here")
	}
	if model.runtime.Machine() != before {
		t.Fatal("a failed commit replaced the cached machine snapshot")
	}
	if before.NamedLensByID(template.ID) != nil {
		t.Fatal("a failed commit added the template to the cached snapshot")
	}
	if out.Lens.Active == nil || out.Lens.Active.LensID != starter.ID {
		t.Fatal("a failed commit applied the template anyway")
	}
}

// The Outliner is the application seam, and it is required before the mutation
// rather than after it: a template that cannot be applied is never committed.
func TestLensAddTemplateRefusesWithoutAnOutlinerAndWritesNothing(t *testing.T) {
	model := lensOperationApp(t)
	template := addableTemplate(t, model)
	before := len(machineLenses(t, model))
	model.crossOutliner = nil

	result := executeLensValue(model, template.ID)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "Outliner is unavailable") {
		t.Fatalf("add with no Outliner = %v, want an unavailable-Outliner refusal", result.Err)
	}
	if got := len(machineLenses(t, model)); got != before {
		t.Fatalf("a refused add wrote %d records, want %d", got, before)
	}
}
