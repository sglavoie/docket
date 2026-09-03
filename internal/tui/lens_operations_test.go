package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/query"
)

// lensOperationApp wires the production lens registrations against the richer
// two-project scope the lifecycle tests use, so an applied lens narrows a real
// projection rather than an empty one.
func lensOperationApp(t *testing.T) *AppModel {
	t.Helper()
	model := typedTestApp(t)
	if err := model.registerLensOperations(); err != nil {
		t.Fatalf("register lens operations: %v", err)
	}
	model.crossOutliner = lensLifecycleOutliner()
	model.crossOutliner.SetSize(model.width, model.height-2)
	model.views.Register(presentation.ViewCrossProjectOutliner, model.crossOutliner)
	return model
}

// lensOperationContext names one saved lens under the picker capability, which
// is the only context these operations are reachable from.
func lensOperationContext(entity EntityIdentity) CommandContext {
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(CapabilityLensPicker)
	ctx.CurrentEntity = entity
	return ctx
}

func executeLens(
	model *AppModel,
	id OperationID,
	entity EntityIdentity,
	value string,
	confirmed bool,
) OperationExecutionResult {
	request := OperationExecutionRequest{
		ID:        id,
		Context:   lensOperationContext(entity),
		Confirmed: confirmed,
	}
	if value != "" {
		request.Value = value
		request.HasValue = true
	}
	return model.operationExecutor.Execute(request)
}

func machineLenses(t *testing.T, model *AppModel) config.NamedLensList {
	t.Helper()
	machine := model.runtime.Machine()
	if machine == nil {
		t.Fatal("machine-local state is unavailable")
	}
	return machine.NamedLenses
}

func lensNamed(t *testing.T, model *AppModel, name string) config.NamedLens {
	t.Helper()
	for _, lens := range machineLenses(t, model) {
		if lens.Name == name {
			return lens
		}
	}
	t.Fatalf("no saved lens named %q in %v", name, lensNames(machineLenses(t, model)))
	return config.NamedLens{}
}

func lensNames(list config.NamedLensList) []string {
	names := make([]string, 0, len(list))
	for _, lens := range list {
		names = append(names, lens.Name)
	}
	return names
}

// starterLensID is the frozen identity of the first shipped preset. The
// preset-parity assertions edit it through the same operations a user-created
// record goes through, because nothing distinguishes the two at rest.
func starterLensID(t *testing.T) string {
	t.Helper()
	starters := config.StarterNamedLenses()
	if len(starters) == 0 {
		t.Fatal("expected shipped starter lenses")
	}
	return starters[0].ID
}

func TestLensOperationsAreReachableOnlyFromTheLensPicker(t *testing.T) {
	model := lensOperationApp(t)
	entity := LensEntity(starterLensID(t))

	for _, id := range []OperationID{
		operationLensApply,
		operationLensCreate,
		operationLensAddTemplate,
		operationLensRename,
		operationLensReplace,
		operationLensDelete,
	} {
		ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
		ctx.CurrentEntity = entity
		result := model.operationExecutor.Execute(OperationExecutionRequest{
			ID:        id,
			Context:   ctx,
			Value:     "Anything",
			HasValue:  true,
			Confirmed: true,
		})
		if !errors.Is(result.Err, ErrOperationUnavailable) {
			t.Fatalf("%s without the picker capability = %v, want unavailable", id, result.Err)
		}
	}
}

func TestLensTargetedOperationsRefuseANonLensCursor(t *testing.T) {
	model := lensOperationApp(t)
	task := TaskEntity("alpha", "backlog")

	for _, id := range []OperationID{
		operationLensApply,
		operationLensRename,
		operationLensReplace,
		operationLensDelete,
	} {
		result := executeLens(model, id, task, "Anything", true)
		if !errors.Is(result.Err, ErrOperationUnavailable) {
			t.Fatalf("%s on a task cursor = %v, want unavailable", id, result.Err)
		}
	}

	// Create resolves no target at all, so the same cursor leaves it available.
	if result := executeLens(model, operationLensCreate, task, "From a task row", false); result.Err != nil {
		t.Fatalf("create on a task cursor = %v, want success", result.Err)
	}
	// Adding a built-in view resolves no target either: the value names the
	// template, so the cursor is nothing to it.
	template := addableTemplate(t, model)
	if result := executeLens(model, operationLensAddTemplate, task, template.ID, false); result.Err != nil {
		t.Fatalf("add template on a task cursor = %v, want success", result.Err)
	}
}

func TestLensOperationsRejectAStaleLensID(t *testing.T) {
	model := lensOperationApp(t)
	missing := LensEntity(lensTestID(42))

	for _, id := range []OperationID{
		operationLensApply,
		operationLensRename,
		operationLensReplace,
		operationLensDelete,
	} {
		result := executeLens(model, id, missing, "Anything", true)
		if result.Err == nil || !strings.Contains(result.Err.Error(), "no longer available") {
			t.Fatalf("%s on a stale lens = %v, want an actionable stale-target error", id, result.Err)
		}
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Fatalf("stale targets changed the saved list to %d records", got)
	}
}

func TestLensCreateSavesTheCurrentViewAndMakesItActive(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	out.selectOutlinerMode(chrome.ModeBacklog)
	out.treeSort = query.SortState{Field: query.SortByTitle, Ascending: true}
	before := len(machineLenses(t, model))

	result := executeLens(model, operationLensCreate, EntityIdentity{}, "Backlog by title", false)
	if result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}
	if got := len(machineLenses(t, model)); got != before+1 {
		t.Fatalf("saved list = %d records, want %d", got, before+1)
	}

	saved := lensNamed(t, model, "Backlog by title")
	if err := config.ValidateNamedLensID(saved.ID); err != nil {
		t.Fatalf("minted lens ID %q: %v", saved.ID, err)
	}
	if saved.Mode != config.LensModeBacklog {
		t.Fatalf("captured mode = %q, want %q", saved.Mode, config.LensModeBacklog)
	}
	if saved.Sort.Field != config.LensSortTitle || !saved.Sort.Ascending {
		t.Fatalf("captured sort = %+v, want an ascending title sort", saved.Sort)
	}
	if !out.hasActiveLens() {
		t.Fatal("expected the new lens to be active")
	}
	if out.activeLens.lensID != saved.ID {
		t.Fatalf("active lens = %q, want the new record %q", out.activeLens.lensID, saved.ID)
	}
	if out.lensModified() {
		t.Fatal("a lens saved from the current view starts unmodified")
	}
	// Machine-local state rides no workspace transaction, so nothing here may
	// reach the executor's undo ledger.
	if undo := model.operationExecutor.Undo(); undo.Err == nil {
		t.Fatal("expected no undo entry for a machine-local lens mutation")
	}
}

// A lens saved from inside another lens must not stack its baseline: one Esc
// still returns to the view the user was in before any lens was applied.
func TestLensCreateRetainsAnExistingBaseline(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	out.selectOutlinerMode(chrome.ModeInbox)
	origin := out.captureOutlinerViewSnapshot()
	applyLensOrFatal(t, out, waitingLens(lensTestID(1), "Waiting", "needle"))

	if result := executeLens(model, operationLensCreate, EntityIdentity{}, "Narrowed", false); result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}
	if out.activeLens.origin.presentation.mode != origin.presentation.mode {
		t.Fatalf(
			"baseline mode = %v, want the pre-lens %v",
			out.activeLens.origin.presentation.mode,
			origin.presentation.mode,
		)
	}
	out.restoreLensBaseline()
	if out.hasActiveLens() || out.primaryMode != chrome.ModeInbox {
		t.Fatalf("one restore left mode %v with lens=%v", out.primaryMode, out.hasActiveLens())
	}
}

func TestLensNamesAreNormalizedAndUniqueCaseInsensitively(t *testing.T) {
	model := lensOperationApp(t)
	existing := lensNamed(t, model, config.StarterNamedLenses()[0].Name)

	result := executeLens(model, operationLensCreate, EntityIdentity{}, "  "+strings.ToUpper(existing.Name)+"  ", false)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "already exists") {
		t.Fatalf("duplicate name = %v, want a name-collision refusal", result.Err)
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Fatalf("a refused create wrote %d records", got)
	}

	if result := executeLens(model, operationLensCreate, EntityIdentity{}, "  Trimmed  ", false); result.Err != nil {
		t.Fatalf("create with padding: %v", result.Err)
	}
	lensNamed(t, model, "Trimmed")
}

func TestLensRenamePreservesTheIDAndTheProjection(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)
	fingerprint := out.activeLens.savedFingerprint
	mode := out.primaryMode
	query := out.lensQuery

	if result := executeLens(model, operationLensRename, LensEntity(starter.ID), "Blocked", false); result.Err != nil {
		t.Fatalf("rename lens: %v", result.Err)
	}

	renamed := lensNamed(t, model, "Blocked")
	if renamed.ID != starter.ID {
		t.Fatalf("rename changed the ID to %q, want %q", renamed.ID, starter.ID)
	}
	if out.activeLens == nil || out.activeLens.name != "Blocked" {
		t.Fatalf("active label = %q, want the new name", out.activeLensLabel())
	}
	if out.activeLens.savedFingerprint != fingerprint ||
		out.primaryMode != mode || out.lensQuery != query {
		t.Fatal("rename changed projection state")
	}
}

func TestLensRenameRefusesADuplicateNameWithoutTouchingAnything(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starters := config.StarterNamedLenses()
	if len(starters) < 2 {
		t.Skip("preset parity needs two shipped starters")
	}
	first := lensNamed(t, model, starters[0].Name)
	applyLensOrFatal(t, out, first)

	result := executeLens(model, operationLensRename, LensEntity(first.ID), starters[1].Name, false)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "already exists") {
		t.Fatalf("duplicate rename = %v, want a name-collision refusal", result.Err)
	}
	if got := lensNamed(t, model, starters[0].Name); got.ID != first.ID {
		t.Fatal("a refused rename rewrote the saved record")
	}
	if out.activeLens.name != first.Name {
		t.Fatalf("a refused rename relabeled the active lens to %q", out.activeLens.name)
	}
}

func TestLensReplaceOverwritesUnderTheSameIDAndClearsTheModifiedMarker(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)

	out.selectOutlinerMode(chrome.ModeBacklog)
	if !out.lensModified() {
		t.Fatal("expected the diverged view to mark the lens modified")
	}

	if result := executeLens(model, operationLensReplace, LensEntity(starter.ID), "", true); result.Err != nil {
		t.Fatalf("replace lens: %v", result.Err)
	}

	replaced := lensNamed(t, model, starter.Name)
	if replaced.ID != starter.ID {
		t.Fatalf("replace changed the ID to %q, want %q", replaced.ID, starter.ID)
	}
	if replaced.Mode != config.LensModeBacklog {
		t.Fatalf("replaced mode = %q, want the current view's %q", replaced.Mode, config.LensModeBacklog)
	}
	if out.lensModified() {
		t.Fatal("replacing the active lens must clear the modified marker")
	}
	if !strings.HasSuffix(out.activeLensLabel(), starter.Name) {
		t.Fatalf("active label = %q, want an unmarked %q", out.activeLensLabel(), starter.Name)
	}
}

func TestLensDeleteRemovesTheDefinitionAndRestoresTheBaseline(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	out.selectOutlinerMode(chrome.ModeInbox)
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)

	if result := executeLens(model, operationLensDelete, LensEntity(starter.ID), "", true); result.Err != nil {
		t.Fatalf("delete lens: %v", result.Err)
	}
	if model.runtime.Machine().NamedLensByID(starter.ID) != nil {
		t.Fatal("delete left the definition in place")
	}
	if out.hasActiveLens() {
		t.Fatal("deleting the active lens must exit it")
	}
	if out.primaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", out.primaryMode, chrome.ModeInbox)
	}
}
