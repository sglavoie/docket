package tui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
		operationLensAddTemplate,
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

// Saving from accepted ordinary-search results must carry the query the visible
// rows were produced by. The projection itself is still replaced — a search
// cannot survive underneath a lens — but the query axis is the user's, and a
// lens saved with an empty one would reopen a view they never saw.
func TestLensCreateCapturesAcceptedSearchQuery(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner

	out.SetSearchQuery("needle")
	if !out.hasSearchProjection() || out.IsSearchMode() {
		t.Fatal("expected an accepted ordinary-search projection with no open input")
	}
	searchRows := slices.Clone(searchVisibleTaskIDs(out))
	if len(searchRows) == 0 {
		t.Fatal("expected the accepted search to hold results")
	}

	if result := executeLens(model, operationLensCreate, EntityIdentity{}, "From search", false); result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}

	saved := lensNamed(t, model, "From search")
	if saved.Query != "needle" {
		t.Fatalf("captured query = %q, want the accepted search query %q", saved.Query, "needle")
	}
	if !out.hasActiveLens() || out.lensQuery != "needle" {
		t.Fatalf("applied lens query = %q with lens=%v", out.lensQuery, out.hasActiveLens())
	}
	if out.hasSearchProjection() {
		t.Fatal("a lens must not leave an ordinary search projection underneath it")
	}
	if out.lensModified() {
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
	if !RunEscCascade(out.escSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if out.hasActiveLens() || out.lensQuery != "" {
		t.Fatalf("Esc left lens=%v query=%q", out.hasActiveLens(), out.lensQuery)
	}
	if !out.hasSearchProjection() || out.SearchQuery() != "needle" {
		t.Fatalf("restored search projection=%v query=%q", out.hasSearchProjection(), out.SearchQuery())
	}
	if got := searchVisibleTaskIDs(out); !slices.Equal(got, searchRows) {
		t.Fatalf("restored rows = %v, want the accepted search rows %v", got, searchRows)
	}

	// Replace reads the same axis, so overwriting a saved lens from accepted
	// results records the query rather than dropping it.
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	if result := executeLens(model, operationLensReplace, LensEntity(starter.ID), "", true); result.Err != nil {
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

// executeLensValue is the typed-value form of executeLens: the template picker
// hands back an ID, and the malformed-payload cases need to deliver something
// that is not a string at all.
func executeLensValue(model *AppModel, id OperationID, value any) OperationExecutionResult {
	return model.operationExecutor.Execute(OperationExecutionRequest{
		ID:       id,
		Context:  lensOperationContext(EntityIdentity{}),
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
	descriptor, ok := model.operationRegistry.Descriptor(operationLensAddTemplate)
	if !ok {
		t.Fatal("the add-template operation is not registered")
	}
	if descriptor.ID != "tui.lens.add-template" {
		t.Fatalf("descriptor ID = %q, want the stable tui.lens.add-template", descriptor.ID)
	}
	if descriptor.Label != "Add built-in view" {
		t.Fatalf("descriptor label = %q", descriptor.Label)
	}
	if descriptor.Category != OperationCategoryFilter {
		t.Fatalf("descriptor category = %q, want the filter family", descriptor.Category)
	}
	if descriptor.TargetSource != OperationTargetGlobal ||
		descriptor.TargetCardinality != TargetCardinalityNone {
		t.Fatalf(
			"descriptor targets %q/%q, want a global zero-target operation",
			descriptor.TargetSource,
			descriptor.TargetCardinality,
		)
	}
	if !descriptor.RequiredCapabilities.Has(CapabilityLensPicker) {
		t.Fatal("descriptor does not require the lens picker capability")
	}
	if descriptor.Risk != OperationRiskMutation {
		t.Fatalf("descriptor risk = %q, want mutation", descriptor.Risk)
	}
	if descriptor.ValueProviderID != ValueProviderLensTemplate {
		t.Fatalf("descriptor value provider = %q, want %q", descriptor.ValueProviderID, ValueProviderLensTemplate)
	}
	// The template is picked from a closed catalog, so it must not route to the
	// typed-text prompt the lens name uses.
	if descriptor.ValueProviderID.textEntry() {
		t.Fatal("the template provider must be picked, not typed")
	}
}

func TestLensAddTemplateAsksForATemplateBeforeItWrites(t *testing.T) {
	model := lensOperationApp(t)
	before := len(machineLenses(t, model))

	result := executeLens(model, operationLensAddTemplate, EntityIdentity{}, "", false)
	if result.ValueRequired == nil || result.ValueRequired.Provider != ValueProviderLensTemplate {
		t.Fatalf("add template ran without asking for a template: %#v", result.ValueRequired)
	}
	if got := len(machineLenses(t, model)); got != before {
		t.Fatalf("an unanswered template prompt wrote %d records, want %d", got, before)
	}
}

func TestLensAddTemplatePersistsTheCatalogDefinitionAndAppliesIt(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	out.selectOutlinerMode(ModeInbox)
	template := addableTemplate(t, model)
	before := len(machineLenses(t, model))

	result := executeLensValue(model, operationLensAddTemplate, template.ID)
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
	if !out.hasActiveLens() || out.activeLens.lensID != template.ID {
		t.Fatalf("added template is not the live lens: %v", out.activeLensLabel())
	}
	if out.lensModified() {
		t.Fatal("a freshly added built-in view starts unmodified")
	}
	// One Esc still returns to the pre-lens view, so the application kept the
	// baseline rather than stacking a new one.
	out.restoreLensBaseline()
	if out.hasActiveLens() || out.primaryMode != ModeInbox {
		t.Fatalf("one restore left mode %v with lens=%v", out.primaryMode, out.hasActiveLens())
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
			result := executeLensValue(model, operationLensAddTemplate, testCase.value)
			if result.Err == nil || !strings.Contains(result.Err.Error(), testCase.want) {
				t.Fatalf("add with %s = %v, want an error naming %q", testCase.name, result.Err, testCase.want)
			}
			if got := len(machineLenses(t, model)); got != before {
				t.Fatalf("a refused add wrote %d records, want %d", got, before)
			}
			if model.crossOutliner.hasActiveLens() {
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

	if result := executeLensValue(model, operationLensAddTemplate, template.ID); result.Err != nil {
		t.Fatalf("add built-in view: %v", result.Err)
	}
	added := len(machineLenses(t, model))

	result := executeLensValue(model, operationLensAddTemplate, template.ID)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "already added as") {
		t.Fatalf("re-adding a template = %v, want an already-added refusal", result.Err)
	}

	if renamed := executeLens(
		model, operationLensRename, LensEntity(template.ID), "Renamed by hand", false,
	); renamed.Err != nil {
		t.Fatalf("rename the added template: %v", renamed.Err)
	}
	result = executeLensValue(model, operationLensAddTemplate, template.ID)
	if result.Err == nil || !strings.Contains(result.Err.Error(), `already added as "Renamed by hand"`) {
		t.Fatalf("re-adding a renamed template = %v, want a refusal naming its current name", result.Err)
	}
	if got := len(machineLenses(t, model)); got != added {
		t.Fatalf("refused re-adds left %d records, want %d", got, added)
	}

	// With the stable ID freed, the template is addable again — an added record is
	// an ordinary lens and nothing more.
	if deleted := executeLens(
		model, operationLensDelete, LensEntity(template.ID), "", true,
	); deleted.Err != nil {
		t.Fatalf("delete the added template: %v", deleted.Err)
	}
	if result := executeLensValue(model, operationLensAddTemplate, template.ID); result.Err != nil {
		t.Fatalf("re-adding a deleted template = %v, want success", result.Err)
	}
}

func TestLensAddTemplateRefusesANameAnotherLensAlreadyAnswersTo(t *testing.T) {
	model := lensOperationApp(t)
	template := addableTemplate(t, model)

	// A user lens minted under its own UUID, wearing the template's name in a
	// different case: uniqueness is case-insensitive, so the template cannot land.
	if created := executeLens(
		model, operationLensCreate, EntityIdentity{}, strings.ToUpper(template.Name), false,
	); created.Err != nil {
		t.Fatalf("create the colliding lens: %v", created.Err)
	}
	before := len(machineLenses(t, model))

	result := executeLensValue(model, operationLensAddTemplate, template.ID)
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

	if result := executeLensValue(model, operationLensAddTemplate, template.ID); result.Err != nil {
		t.Fatalf("add built-in view: %v", result.Err)
	}
	if renamed := executeLens(
		model, operationLensRename, LensEntity(template.ID), "Renamed by hand", false,
	); renamed.Err != nil {
		t.Fatalf("rename the added template: %v", renamed.Err)
	}
	// The template's own name is free again, so a user lens can take it.
	if created := executeLens(
		model, operationLensCreate, EntityIdentity{}, template.Name, false,
	); created.Err != nil {
		t.Fatalf("create the name-colliding lens: %v", created.Err)
	}

	result := executeLensValue(model, operationLensAddTemplate, template.ID)
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
	result := executeLensValue(model, operationLensAddTemplate, template.ID)
	if result.Err == nil {
		t.Skip("the runtime root stayed writable; the failure path is untested here")
	}
	if model.runtime.Machine() != before {
		t.Fatal("a failed commit replaced the cached machine snapshot")
	}
	if before.NamedLensByID(template.ID) != nil {
		t.Fatal("a failed commit added the template to the cached snapshot")
	}
	if out.activeLens == nil || out.activeLens.lensID != starter.ID {
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

	result := executeLensValue(model, operationLensAddTemplate, template.ID)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "Outliner is unavailable") {
		t.Fatalf("add with no Outliner = %v, want an unavailable-Outliner refusal", result.Err)
	}
	if got := len(machineLenses(t, model)); got != before {
		t.Fatalf("a refused add wrote %d records, want %d", got, before)
	}
}
