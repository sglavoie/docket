package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
	model.views.Register(ViewCrossProjectOutliner, model.crossOutliner)
	return model
}

// lensOperationContext names one saved lens under the picker capability, which
// is the only context these operations are reachable from.
func lensOperationContext(entity EntityIdentity) CommandContext {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
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
		operationLensRename,
		operationLensReplace,
		operationLensDelete,
	} {
		ctx := PresentationCommandContext(ViewCrossProjectOutliner)
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
	out.selectOutlinerMode(ModeBacklog)
	out.treeSort = SortState{Field: SortByTitle, Ascending: true}
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
	out.selectOutlinerMode(ModeInbox)
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
	if out.hasActiveLens() || out.primaryMode != ModeInbox {
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

	out.selectOutlinerMode(ModeBacklog)
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
	out.selectOutlinerMode(ModeInbox)
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
	if out.primaryMode != ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", out.primaryMode, ModeInbox)
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

	if result := executeLens(model, operationLensDelete, LensEntity(other.ID), "", true); result.Err != nil {
		t.Fatalf("delete lens: %v", result.Err)
	}
	if !out.hasActiveLens() || out.activeLens.lensID != active.ID {
		t.Fatal("deleting another lens dismissed the active one")
	}
}

func TestLensReplaceAndDeleteConfirmBeforeTheyWrite(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)
	out.selectOutlinerMode(ModeBacklog)

	for _, id := range []OperationID{operationLensReplace, operationLensDelete} {
		result := executeLens(model, id, LensEntity(starter.ID), "", false)
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
		if !out.hasActiveLens() || out.activeLens.lensID != starter.ID {
			t.Fatalf("%s changed the active lens before confirmation", id)
		}
	}
}

func TestLensRenameAndCreateAskForANameBeforeTheyWrite(t *testing.T) {
	model := lensOperationApp(t)
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)

	for _, entity := range []struct {
		id     OperationID
		entity EntityIdentity
	}{
		{operationLensCreate, EntityIdentity{}},
		{operationLensRename, LensEntity(starter.ID)},
	} {
		result := executeLens(model, entity.id, entity.entity, "", false)
		if result.ValueRequired == nil || result.ValueRequired.Provider != ValueProviderLensName {
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

	prompt, err := model.newOperationTextPrompt(OperationExecutionRequest{
		ID:      operationLensRename,
		Context: lensOperationContext(LensEntity(starter.ID)),
	}, ValueProviderLensName)
	if err != nil {
		t.Fatalf("rename prompt: %v", err)
	}
	if prompt.input.Value() != starter.Name {
		t.Fatalf("rename prompt prefilled %q, want %q", prompt.input.Value(), starter.Name)
	}

	create, err := model.newOperationTextPrompt(OperationExecutionRequest{
		ID:      operationLensCreate,
		Context: lensOperationContext(EntityIdentity{}),
	}, ValueProviderLensName)
	if err != nil {
		t.Fatalf("create prompt: %v", err)
	}
	if create.input.Value() != "" {
		t.Fatalf("create prompt prefilled %q, want an empty buffer", create.input.Value())
	}

	if _, err := model.newOperationTextPrompt(OperationExecutionRequest{
		ID:      operationLensRename,
		Context: lensOperationContext(LensEntity(lensTestID(42))),
	}, ValueProviderLensName); err == nil {
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
		if result := executeLens(model, operationLensRename, LensEntity(saved.ID), renamed, false); result.Err != nil {
			t.Fatalf("rename starter %q: %v", starter.Name, result.Err)
		}
		if result := executeLens(model, operationLensReplace, LensEntity(saved.ID), "", true); result.Err != nil {
			t.Fatalf("replace starter %q: %v", starter.Name, result.Err)
		}
		if got := lensNamed(t, model, renamed); got.ID != saved.ID {
			t.Fatalf("editing starter %q minted a new identity", starter.Name)
		}
		if result := executeLens(model, operationLensDelete, LensEntity(saved.ID), "", true); result.Err != nil {
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

// With no writable workspace rooted, the in-memory seam is the whole commit and
// the runtime accessors must observe it exactly as they would after a write.
func TestLensOperationsUseTheInMemorySeamWithoutADataDir(t *testing.T) {
	model := lensOperationApp(t)
	if model.runtime.DataDir() != "" {
		t.Fatalf("fixture rooted a data dir at %q", model.runtime.DataDir())
	}
	before := model.runtime.Machine()

	if result := executeLens(model, operationLensCreate, EntityIdentity{}, "In memory", false); result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}
	after := model.runtime.Machine()
	if after == before {
		t.Fatal("the in-memory seam must publish the clone, not mutate the cached snapshot")
	}
	if before.NamedLensByID(lensNamed(t, model, "In memory").ID) != nil {
		t.Fatal("the pre-commit snapshot was mutated in place")
	}
	if after.Theme != before.Theme {
		t.Fatalf("the clone dropped the theme: %q vs %q", after.Theme, before.Theme)
	}
}

// The disk path is the production one: the record must survive a fresh load of
// the same workspace's machine-local file.
func TestLensCreatePersistsToTheMachineLocalFile(t *testing.T) {
	model := lensOperationApp(t)
	dataDir := t.TempDir()
	project := models.Project{ID: "alpha", Name: "alpha", Slug: "alpha"}
	model.runtime = testRuntimeAt(dataDir, testConfigWithProjects(&project))
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	// testRuntimeAt seeds the catalog alone; a reload reads the split-domain set
	// whole, so the planning file has to exist beside it.
	if err := manager.CommitPlanning(config.NewPlanningState()); err != nil {
		t.Fatalf("seed planning: %v", err)
	}

	if result := executeLens(model, operationLensCreate, EntityIdentity{}, "Persisted", false); result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}

	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("reload domains: %v", err)
	}
	if domains.Machine == nil {
		t.Fatal("reloaded domains carry no machine state")
	}
	if lensNames(domains.Machine.NamedLenses) == nil ||
		!strings.Contains(strings.Join(lensNames(domains.Machine.NamedLenses), ","), "Persisted") {
		t.Fatalf("reloaded lenses = %v, want the saved record", lensNames(domains.Machine.NamedLenses))
	}
}

// A commit that fails on disk must leave the cached snapshot and the active
// lens exactly as they were: the effect is published only after success.
func TestAFailedLensCommitChangesNeitherMachineStateNorTheActiveLens(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	dataDir := t.TempDir()
	project := models.Project{ID: "alpha", Name: "alpha", Slug: "alpha"}
	model.runtime = testRuntimeAt(dataDir, testConfigWithProjects(&project))

	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)

	// A read-only runtime root is the one failure the commit cannot route around.
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Chmod(home, 0o500); err != nil {
		t.Skipf("cannot make the runtime root read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	before := model.runtime.Machine()
	result := executeLens(model, operationLensRename, LensEntity(starter.ID), "Renamed", false)
	if result.Err == nil {
		t.Skip("the runtime root stayed writable; the failure path is untested here")
	}
	if model.runtime.Machine() != before {
		t.Fatal("a failed commit replaced the cached machine snapshot")
	}
	if before.NamedLensByID(starter.ID).Name != starter.Name {
		t.Fatal("a failed commit rewrote the cached definition in place")
	}
	if out.activeLens == nil || out.activeLens.name != starter.Name {
		t.Fatal("a failed commit relabeled the active lens")
	}
}
