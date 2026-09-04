package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// Saving from accepted ordinary-search results must carry the query the visible
// rows were produced by. The projection itself is still replaced — a search
// cannot survive underneath a lens — but the query axis is the user's, and a
// lens saved with an empty one would reopen a view they never saw.
func TestLensCreateCapturesAcceptedSearchQuery(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner

	out.SetSearchQuery("needle")
	if !out.HasSearchProjection() || out.IsSearchMode() {
		t.Fatal("expected an accepted ordinary-search projection with no open input")
	}
	searchRows := slices.Clone(searchVisibleTaskIDs(out))
	if len(searchRows) == 0 {
		t.Fatal("expected the accepted search to hold results")
	}

	if result := executeLens(model, lens.OperationLensCreate, operation.EntityIdentity{}, "From search", false); result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}

	saved := lensNamed(t, model, "From search")
	if saved.Query != "needle" {
		t.Fatalf("captured query = %q, want the accepted search query %q", saved.Query, "needle")
	}
	if !out.HasActiveLens() || out.Lens.Query != "needle" {
		t.Fatalf("applied lens query = %q with lens=%v", out.Lens.Query, out.HasActiveLens())
	}
	if out.HasSearchProjection() {
		t.Fatal("a lens must not leave an ordinary search projection underneath it")
	}
	if out.LensModified() {
		t.Fatal("a lens saved from the current view starts unmodified")
	}
	narrowed := searchVisibleTaskIDs(out)
	if len(narrowed) == 0 {
		t.Fatal("expected the saved query to narrow the lens projection")
	}
	for _, id := range narrowed {
		if !slices.Contains(searchRows, id) {
			t.Fatalf("lens row %q was not one of the accepted search results %v", id, searchRows)
		}
	}

	// One Esc hands the pre-lens frame back exactly, search included.
	if !listsurface.RunEscCascade(out.EscSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if out.HasActiveLens() || out.Lens.Query != "" {
		t.Fatalf("Esc left lens=%v query=%q", out.HasActiveLens(), out.Lens.Query)
	}
	if !out.HasSearchProjection() || out.SearchQuery() != "needle" {
		t.Fatalf("restored search projection=%v query=%q", out.HasSearchProjection(), out.SearchQuery())
	}
	if got := searchVisibleTaskIDs(out); !slices.Equal(got, searchRows) {
		t.Fatalf("restored rows = %v, want the accepted search rows %v", got, searchRows)
	}

	// Replace reads the same axis, so overwriting a saved lens from accepted
	// results records the query rather than dropping it.
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	if result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(starter.ID), "", true); result.Err != nil {
		t.Fatalf("replace lens: %v", result.Err)
	}
	if replaced := lensNamed(t, model, starter.Name); replaced.Query != "needle" {
		t.Fatalf("replaced query = %q, want the accepted search query %q", replaced.Query, "needle")
	}
}

func TestDeletingAnInactiveLensLeavesTheActiveOneAlone(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starters := config.StarterNamedLenses()
	if len(starters) < 2 {
		t.Skip("this case needs two shipped starters")
	}
	active := lensNamed(t, model, starters[0].Name)
	applyLensOrFatal(t, out, active)
	other := lensNamed(t, model, starters[1].Name)

	if result := executeLens(model, lens.OperationLensDelete, operation.LensEntity(other.ID), "", true); result.Err != nil {
		t.Fatalf("delete lens: %v", result.Err)
	}
	if !out.HasActiveLens() || out.Lens.Active.LensID != active.ID {
		t.Fatal("deleting another lens dismissed the active one")
	}
}

func TestLensReplaceAndDeleteConfirmBeforeTheyWrite(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)
	out.SelectOutlinerMode(chrome.ModeBacklog)

	for _, id := range []operation.OperationID{lens.OperationLensReplace, lens.OperationLensDelete} {
		result := executeLens(model, id, operation.LensEntity(starter.ID), "", false)
		if result.ConfirmationRequired == nil {
			t.Fatalf("%s ran without asking for confirmation", id)
		}
		if result.ConfirmationRequired.Label == "" {
			t.Fatalf("%s asked for confirmation without naming itself", id)
		}
		// Cancellation is simply never sending the confirmed request, so the
		// unconfirmed pass must leave both tiers exactly as they were.
		saved := lensNamed(t, model, starter.Name)
		if saved.Mode != starter.Mode {
			t.Fatalf("%s rewrote the saved record before confirmation", id)
		}
		if !out.HasActiveLens() || out.Lens.Active.LensID != starter.ID {
			t.Fatalf("%s changed the active lens before confirmation", id)
		}
	}
}

func TestLensRenameAndCreateAskForANameBeforeTheyWrite(t *testing.T) {
	model := lensOperationApp(t)
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)

	for _, entity := range []struct {
		id     operation.OperationID
		entity operation.EntityIdentity
	}{
		{lens.OperationLensCreate, operation.EntityIdentity{}},
		{lens.OperationLensRename, operation.LensEntity(starter.ID)},
	} {
		result := executeLens(model, entity.id, entity.entity, "", false)
		if result.ValueRequired == nil || result.ValueRequired.Provider != operation.ValueProviderLensName {
			t.Fatalf("%s ran without asking for a name: %#v", entity.id, result.ValueRequired)
		}
	}
	if got := len(machineLenses(t, model)); got != len(config.StarterNamedLenses()) {
		t.Fatalf("an unanswered name prompt wrote %d records", got)
	}
}

// The name prompt is built from the frozen identity, so a rename whose target
// vanished refuses at the prompt rather than after the user types.
func TestLensNamePromptPrefillsTheRenameTargetAndRefusesAStaleOne(t *testing.T) {
	model := lensOperationApp(t)
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)

	prompt, err := model.newOperationTextPrompt(operation.OperationExecutionRequest{
		ID:      lens.OperationLensRename,
		Context: lensOperationContext(operation.LensEntity(starter.ID)),
	}, operation.ValueProviderLensName)
	if err != nil {
		t.Fatalf("rename prompt: %v", err)
	}
	if prompt.input.Value() != starter.Name {
		t.Fatalf("rename prompt prefilled %q, want %q", prompt.input.Value(), starter.Name)
	}

	create, err := model.newOperationTextPrompt(operation.OperationExecutionRequest{
		ID:      lens.OperationLensCreate,
		Context: lensOperationContext(operation.EntityIdentity{}),
	}, operation.ValueProviderLensName)
	if err != nil {
		t.Fatalf("create prompt: %v", err)
	}
	if create.input.Value() != "" {
		t.Fatalf("create prompt prefilled %q, want an empty buffer", create.input.Value())
	}

	if _, err := model.newOperationTextPrompt(operation.OperationExecutionRequest{
		ID:      lens.OperationLensRename,
		Context: lensOperationContext(operation.LensEntity(lensTestID(42))),
	}, operation.ValueProviderLensName); err == nil {
		t.Fatal("expected a stale rename target to refuse at the prompt")
	}
}

// Presets are ordinary records: the same three edits reach them, and the list
// they leave behind is empty rather than reseeded.
func TestStarterLensesAreEditableAndRemovableLikeUserRecords(t *testing.T) {
	model := lensOperationApp(t)
	starters := config.StarterNamedLenses()

	for i, starter := range starters {
		saved := lensNamed(t, model, starter.Name)
		if saved.ID != starter.ID {
			t.Fatalf("starter %q loaded under %q, want the frozen %q", starter.Name, saved.ID, starter.ID)
		}
		renamed := "Preset " + strings.Repeat("x", i+1)
		if result := executeLens(model, lens.OperationLensRename, operation.LensEntity(saved.ID), renamed, false); result.Err != nil {
			t.Fatalf("rename starter %q: %v", starter.Name, result.Err)
		}
		if result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(saved.ID), "", true); result.Err != nil {
			t.Fatalf("replace starter %q: %v", starter.Name, result.Err)
		}
		if got := lensNamed(t, model, renamed); got.ID != saved.ID {
			t.Fatalf("editing starter %q minted a new identity", starter.Name)
		}
		if result := executeLens(model, lens.OperationLensDelete, operation.LensEntity(saved.ID), "", true); result.Err != nil {
			t.Fatalf("delete starter %q: %v", starter.Name, result.Err)
		}
	}

	remaining := machineLenses(t, model)
	if len(remaining) != 0 {
		t.Fatalf("deleting every starter left %v", lensNames(remaining))
	}
	if remaining == nil {
		t.Fatal("an emptied list must stay non-nil so it persists as [] rather than reseeding")
	}
}
