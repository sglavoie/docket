package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/lens"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/route"
)

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
	entity := operation.LensEntity(starterLensID(t))

	for _, id := range []operation.OperationID{
		lens.OperationLensApply,
		lens.OperationLensCreate,
		lens.OperationLensAddTemplate,
		lens.OperationLensRename,
		lens.OperationLensDuplicate,
		lens.OperationLensReplace,
		lens.OperationLensDelete,
	} {
		ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
		ctx.CurrentEntity = entity
		result := model.operations.Executor.Execute(operation.OperationExecutionRequest{
			ID:        id,
			Context:   ctx,
			Value:     "Anything",
			HasValue:  true,
			Confirmed: true,
		})
		if !errors.Is(result.Err, operation.ErrOperationUnavailable) {
			t.Fatalf("%s without the picker capability = %v, want unavailable", id, result.Err)
		}
	}
}

func TestLensTargetedOperationsRefuseANonLensCursor(t *testing.T) {
	model := lensOperationApp(t)
	task := operation.TaskEntity("alpha", "backlog")

	for _, id := range []operation.OperationID{
		lens.OperationLensApply,
		lens.OperationLensRename,
		lens.OperationLensDuplicate,
		lens.OperationLensReplace,
		lens.OperationLensDelete,
	} {
		result := executeLens(model, id, task, "Anything", true)
		if !errors.Is(result.Err, operation.ErrOperationUnavailable) {
			t.Fatalf("%s on a task cursor = %v, want unavailable", id, result.Err)
		}
	}

	// Create resolves no target at all, so the same cursor leaves it available.
	if result := executeLens(model, lens.OperationLensCreate, task, "From a task row", false); result.Err != nil {
		t.Fatalf("create on a task cursor = %v, want success", result.Err)
	}
	// Adding a built-in view resolves no target either: the value names the
	// template, so the cursor is nothing to it.
	template := addableTemplate(t, model)
	if result := executeLens(model, lens.OperationLensAddTemplate, task, template.ID, false); result.Err != nil {
		t.Fatalf("add template on a task cursor = %v, want success", result.Err)
	}
}

func TestLensOperationsRejectAStaleLensID(t *testing.T) {
	model := lensOperationApp(t)
	missing := operation.LensEntity(lensTestID(42))

	for _, id := range []operation.OperationID{
		lens.OperationLensApply,
		lens.OperationLensRename,
		lens.OperationLensDuplicate,
		lens.OperationLensReplace,
		lens.OperationLensDelete,
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
	out.SelectOutlinerMode(chrome.ModeBacklog)
	out.TreeSort = query.SortState{Field: query.SortByTitle, Ascending: true}
	before := len(machineLenses(t, model))

	result := executeLens(model, lens.OperationLensCreate, operation.EntityIdentity{}, "Backlog by title", false)
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
	if !out.HasActiveLens() {
		t.Fatal("expected the new lens to be active")
	}
	if out.Lens.Active.LensID != saved.ID {
		t.Fatalf("active lens = %q, want the new record %q", out.Lens.Active.LensID, saved.ID)
	}
	if out.LensModified() {
		t.Fatal("a lens saved from the current view starts unmodified")
	}
	// Machine-local state rides no workspace transaction, so nothing here may
	// reach the executor's undo ledger.
	if undo := model.operations.Executor.Undo(); undo.Err == nil {
		t.Fatal("expected no undo entry for a machine-local lens mutation")
	}
}

// A lens saved from inside another lens must not stack its baseline: one Esc
// still returns to the view the user was in before any lens was applied.
func TestLensCreateRetainsAnExistingBaseline(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	out.SelectOutlinerMode(chrome.ModeInbox)
	origin := out.CaptureOutlinerViewSnapshot()
	applyLensOrFatal(t, out, waitingLens(lensTestID(1), "Waiting", "needle"))

	if result := executeLens(model, lens.OperationLensCreate, operation.EntityIdentity{}, "Narrowed", false); result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}
	if out.Lens.Active.Origin.Presentation.Projection.Mode != origin.Presentation.Projection.Mode {
		t.Fatalf(
			"baseline mode = %v, want the pre-lens %v",
			out.Lens.Active.Origin.Presentation.Projection.Mode,
			origin.Presentation.Projection.Mode,
		)
	}
	out.RestoreLensBaseline()
	if out.HasActiveLens() || out.PrimaryMode != chrome.ModeInbox {
		t.Fatalf("one restore left mode %v with lens=%v", out.PrimaryMode, out.HasActiveLens())
	}
}

func TestLensNamesAreNormalizedAndUniqueCaseInsensitively(t *testing.T) {
	model := lensOperationApp(t)
	existing := lensNamed(t, model, config.StarterNamedLenses()[0].Name)

	result := executeLens(model, lens.OperationLensCreate, operation.EntityIdentity{}, "  "+strings.ToUpper(existing.Name)+"  ", false)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "already exists") {
		t.Fatalf("duplicate name = %v, want a name-collision refusal", result.Err)
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Fatalf("a refused create wrote %d records", got)
	}

	if result := executeLens(model, lens.OperationLensCreate, operation.EntityIdentity{}, "  Trimmed  ", false); result.Err != nil {
		t.Fatalf("create with padding: %v", result.Err)
	}
	lensNamed(t, model, "Trimmed")
}

func TestLensRenamePreservesTheIDAndTheProjection(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)
	fingerprint := out.Lens.Active.SavedState.Fingerprint()
	mode := out.PrimaryMode
	query := out.Lens.Query

	if result := executeLens(model, lens.OperationLensRename, operation.LensEntity(starter.ID), "Blocked", false); result.Err != nil {
		t.Fatalf("rename lens: %v", result.Err)
	}

	renamed := lensNamed(t, model, "Blocked")
	if renamed.ID != starter.ID {
		t.Fatalf("rename changed the ID to %q, want %q", renamed.ID, starter.ID)
	}
	if out.Lens.Active == nil || out.Lens.Active.Name != "Blocked" {
		t.Fatalf("active label = %q, want the new name", out.ActiveLensLabel())
	}
	if out.Lens.Active.SavedState.Fingerprint() != fingerprint ||
		out.PrimaryMode != mode || out.Lens.Query != query {
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

	result := executeLens(model, lens.OperationLensRename, operation.LensEntity(first.ID), starters[1].Name, false)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "already exists") {
		t.Fatalf("duplicate rename = %v, want a name-collision refusal", result.Err)
	}
	if got := lensNamed(t, model, starters[0].Name); got.ID != first.ID {
		t.Fatal("a refused rename rewrote the saved record")
	}
	if out.Lens.Active.Name != first.Name {
		t.Fatalf("a refused rename relabeled the active lens to %q", out.Lens.Active.Name)
	}
}

func TestLensReplaceOverwritesUnderTheSameIDAndClearsTheModifiedMarker(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)

	out.SelectOutlinerMode(chrome.ModeBacklog)
	if !out.LensModified() {
		t.Fatal("expected the diverged view to mark the lens modified")
	}

	if result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(starter.ID), "", true); result.Err != nil {
		t.Fatalf("replace lens: %v", result.Err)
	}

	replaced := lensNamed(t, model, starter.Name)
	if replaced.ID != starter.ID {
		t.Fatalf("replace changed the ID to %q, want %q", replaced.ID, starter.ID)
	}
	if replaced.Mode != config.LensModeBacklog {
		t.Fatalf("replaced mode = %q, want the current view's %q", replaced.Mode, config.LensModeBacklog)
	}
	if out.LensModified() {
		t.Fatal("replacing the active lens must clear the modified marker")
	}
	if !strings.HasSuffix(out.ActiveLensLabel(), starter.Name) {
		t.Fatalf("active label = %q, want an unmarked %q", out.ActiveLensLabel(), starter.Name)
	}
}

func TestLensDeleteRemovesTheDefinitionAndRestoresTheBaseline(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	out.SelectOutlinerMode(chrome.ModeInbox)
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)

	if result := executeLens(model, lens.OperationLensDelete, operation.LensEntity(starter.ID), "", true); result.Err != nil {
		t.Fatalf("delete lens: %v", result.Err)
	}
	if model.domain.runtime.Machine().NamedLensByID(starter.ID) != nil {
		t.Fatal("delete left the definition in place")
	}
	if out.HasActiveLens() {
		t.Fatal("deleting the active lens must exit it")
	}
	if out.PrimaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", out.PrimaryMode, chrome.ModeInbox)
	}
}
