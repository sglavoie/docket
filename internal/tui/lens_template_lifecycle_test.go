package tui

import (
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// These are the end-to-end journeys for the built-in view catalog: a template is
// added the way a user adds one, and the assertions follow the record from the
// commit through the projection it installs, the ordinary lens lifecycle it then
// lives in, and the ways adding one can fail. Nothing here reaches for a private
// seam the earlier unit tests already own; what is new is that every step is
// driven by the catalog's own definitions rather than by a hand-built lens.

// settleLensPopulationLoad answers the one asynchronous application a lens may
// have started, using the selection the real load command performs against the
// Outliner's own runtime. The catalog holds project metadata alone, so the
// task-bearing fixtures stand in for what the read would have returned; a caller
// asserting only identity can leave them out. It is a no-op for a template that
// applies synchronously, so a caller never has to know which kind it just added.
func settleLensPopulationLoad(t *testing.T, outliner *OutlinerModel, sources ...*models.Project) {
	t.Helper()
	pending := outliner.pendingLens
	if pending == nil {
		return
	}
	selected := selectCrossScopeProjects(outliner.runtime, CrossScopeOpts{
		ActiveOnly:     true,
		FocusProjectID: outliner.focusRequestForLoad(),
		ProjectFilter:  pending.state.projectFilter,
	})
	byID := make(map[string]*models.Project, len(sources))
	for _, source := range sources {
		byID[source.ID] = source
	}
	projects := make([]*models.Project, 0, len(selected))
	for _, project := range selected {
		if loaded, ok := byID[project.ID]; ok {
			projects = append(projects, loaded)
			continue
		}
		projects = append(projects, project)
	}
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects:   projects,
		generation: pending.generation,
	}); !published {
		t.Fatal("the pending population load published nothing")
	}
}

// addTemplateAndSettle adds one template through the production operation and
// settles whatever it started, so the model is quiescent when the caller asserts.
func addTemplateAndSettle(t *testing.T, model *AppModel, template config.NamedLens) {
	t.Helper()
	result := executeLensValue(model, template.ID)
	if result.Err != nil {
		t.Fatalf("add built-in view %q: %v", template.Name, result.Err)
	}
	settleLensPopulationLoad(t, model.crossOutliner)
}

// emptyTemplateMachine frees every catalog identity so a table can add any
// template it names, including the three the starters occupy.
func emptyTemplateMachine(t *testing.T, model *AppModel) {
	t.Helper()
	setMachineLenses(t, model, config.NamedLensList{})
}

// --- the projection each template installs -----------------------------------

// TestEveryBuiltinTemplateInstallsItsExactProjection walks the whole catalog
// through the add operation. The saved record must be the catalog definition byte
// for byte, and the live view must be exactly the projection that definition
// decodes to: the mode it names, both filter families, the population facets, an
// empty narrowing query, the sort reconciled against the mode's ordering policy,
// and a frame whose saved fingerprint marks it unmodified.
func TestEveryBuiltinTemplateInstallsItsExactProjection(t *testing.T) {
	for _, template := range config.BuiltinNamedLensTemplates() {
		t.Run(template.Name, func(t *testing.T) {
			model := lensOperationApp(t)
			emptyTemplateMachine(t, model)
			out := model.crossOutliner
			// A mode no template names, so the restoration assertion below cannot
			// pass by accident.
			out.selectOutlinerMode(ModeInbox)

			want, err := lensStateFromNamedLens(template)
			if err != nil {
				t.Fatalf("the catalog definition does not decode: %v", err)
			}
			mode, ok := OutlinerModeByID(want.mode)
			if !ok {
				t.Fatalf("template mode %q is not a catalog mode", want.mode)
			}

			addTemplateAndSettle(t, model, template)

			saved := model.runtime.Machine().NamedLensByID(template.ID)
			if saved == nil {
				t.Fatalf("no record saved under the template ID %q", template.ID)
			}
			if !reflect.DeepEqual(*saved, template) {
				t.Fatalf("saved record = %+v, want the catalog definition %+v", *saved, template)
			}

			if !out.hasActiveLens() || out.activeLens.lensID != template.ID {
				t.Fatalf("the added template is not the live lens: %v", out.activeLensLabel())
			}
			if out.activeLens.name != template.Name {
				t.Fatalf("frame name = %q, want %q", out.activeLens.name, template.Name)
			}
			if out.primaryMode != want.mode {
				t.Fatalf("mode = %v, want the template's %v", out.primaryMode, want.mode)
			}
			if out.lensQuery != "" {
				t.Fatalf("narrowing query = %q, want the template's empty one", out.lensQuery)
			}
			// The fingerprint is the canonical rendering of every lens-owned axis at
			// once: filters, population facets, query, mode, and sort. Comparing it
			// against the definition's own is what proves the live view is the
			// template rather than merely something like it.
			if got := out.captureLensState().fingerprint(); got != want.fingerprint() {
				t.Fatalf("live projection fingerprint = %q, want %q", got, want.fingerprint())
			}
			if out.activeLens.savedFingerprint != want.fingerprint() {
				t.Fatal("the frame recorded a definition the template never carried")
			}
			if out.lensModified() {
				t.Fatal("a freshly added built-in view starts unmodified")
			}
			// The manual sort the template owns survives as treeSort whatever the
			// mode does with it; only a tree-order mode makes it the active order.
			if out.treeSort != want.sort {
				t.Fatalf("treeSort = %+v, want the template's %+v", out.treeSort, want.sort)
			}
			if mode.SupportsManualSort() {
				if out.sort != want.sort {
					t.Fatalf("active sort = %+v, want the template's %+v", out.sort, want.sort)
				}
			} else if out.sort != NewSortState() {
				t.Fatalf("active sort = %+v, want a fixed-order mode's inactive default", out.sort)
			}
			if out.pendingLens != nil {
				t.Fatal("the application left a load still armed")
			}

			// One Esc, whichever path the template took to get here.
			if !RunEscCascade(out.escSteps()) {
				t.Fatal("expected Esc to consume the added lens")
			}
			if out.hasActiveLens() || out.primaryMode != ModeInbox {
				t.Fatalf("one Esc left mode %v with lens=%v", out.primaryMode, out.hasActiveLens())
			}
		})
	}
}

// TestNoCatalogTemplateCanFailToApplyAfterItsCommit covers the one branch the
// add handler holds for an application that fails after the record is already
// committed. It is unreachable for the shipped catalog by construction — every
// definition validates, names a live mode, and carries no query for the grammar
// to refuse — and this is the assertion that keeps it that way: a template added
// to the catalog that could fail here would fail this test rather than surface as
// a half-applied add in front of a user.
func TestNoCatalogTemplateCanFailToApplyAfterItsCommit(t *testing.T) {
	for _, template := range config.BuiltinNamedLensTemplates() {
		outliner := lensLifecycleOutliner()
		if _, err := outliner.ApplyNamedLens(template); err != nil {
			t.Fatalf("template %q cannot be applied: %v", template.Name, err)
		}
	}
}

// --- the restoration point across a chain of adds ----------------------------

// TestAddingTemplatesFromANormalViewKeepsOneRestorationPoint chains three adds,
// one of which populates asynchronously. A lens never stacks, so however many
// templates replace one another, the baseline is still the view the first one
// replaced and one Esc is still the whole way out.
func TestAddingTemplatesFromANormalViewKeepsOneRestorationPoint(t *testing.T) {
	model := lensOperationApp(t)
	emptyTemplateMachine(t, model)
	out := model.crossOutliner
	out.selectOutlinerMode(ModeBacklog)
	out.collapsedSections["pre-lens-fold"] = true
	baselineProjects := slices.Clone(out.loadedProjects)

	chain := []string{"Overdue", "Active Work", "Parked Work"}
	var origin outlinerViewSnapshot
	for i, name := range chain {
		addTemplateAndSettle(t, model, lensTemplateByName(t, name))
		if i == 0 {
			origin = out.activeLens.origin
			continue
		}
		if out.activeLens.origin.presentation.mode != origin.presentation.mode {
			t.Fatalf("adding %q replaced the original restoration point", name)
		}
	}
	if out.activeLens.name != "Parked Work" {
		t.Fatalf("active lens = %q, want the last one added", out.activeLens.name)
	}

	if !RunEscCascade(out.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if out.hasActiveLens() {
		t.Fatal("one Esc must leave the lens entirely after a chain of adds")
	}
	if out.primaryMode != ModeBacklog {
		t.Fatalf("restored mode = %v, want the pre-lens %v", out.primaryMode, ModeBacklog)
	}
	if !out.collapsedSections["pre-lens-fold"] {
		t.Fatal("restoration lost the fold the pre-lens view held")
	}
	if !slices.Equal(out.loadedProjects, baselineProjects) {
		t.Fatal("restoration did not return the pre-lens population")
	}
}

// TestAddingATemplateFromInsideALensKeepsThePreLensBaseline is the same rule
// entered from the other side: the view a template replaces here is itself a
// lens, and a lens is never the thing Esc restores to.
func TestAddingATemplateFromInsideALensKeepsThePreLensBaseline(t *testing.T) {
	model := lensOperationApp(t)
	emptyTemplateMachine(t, model)
	out := model.crossOutliner
	out.selectOutlinerMode(ModeInbox)

	// A saved user lens first, so the template is added from inside an active one.
	applyLensOrFatal(t, out, waitingLens(lensTestID(1), "Held", "needle"))
	origin := out.activeLens.origin
	if origin.presentation.mode != ModeInbox {
		t.Fatalf("pre-lens baseline mode = %v, want %v", origin.presentation.mode, ModeInbox)
	}

	addTemplateAndSettle(t, model, lensTemplateByName(t, "Parked Work"))
	if out.activeLens.origin.presentation.mode != ModeInbox {
		t.Fatal("adding a template from inside a lens rebased the restoration point onto that lens")
	}
	if out.lensQuery != "" {
		t.Fatalf("the added template inherited the previous lens's query %q", out.lensQuery)
	}

	if !RunEscCascade(out.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if out.hasActiveLens() || out.primaryMode != ModeInbox {
		t.Fatalf("one Esc left mode %v with lens=%v", out.primaryMode, out.hasActiveLens())
	}
}

// --- an added template is an ordinary record ---------------------------------

// templateRowReason reads the gallery's own verdict on one template against the
// machine as it stands, which is the sentence a user sees on the greyed row.
func templateRowReason(t *testing.T, model *AppModel, id string) string {
	t.Helper()
	picker := openLensTemplateGallery(t, model)
	reason := galleryRow(t, picker, id).DisabledReason
	model.operationValuePicker = nil
	return reason
}

// TestAnAddedTemplateIsAnOrdinaryRecordAcrossRenameReplaceAndDelete follows one
// added template through the whole lens lifecycle. Nothing distinguishes it from
// a user-created record: it renames, it is replaced by the current view, and it
// deletes. The only thing the catalog keeps is the stable ID, which is what the
// gallery reads to say a template is already added however far its record has
// since traveled from the definition it started as.
func TestAnAddedTemplateIsAnOrdinaryRecordAcrossRenameReplaceAndDelete(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	template := addableTemplate(t, model)

	if reason := templateRowReason(t, model, template.ID); reason != "" {
		t.Fatalf("an unadded template is disabled: %q", reason)
	}

	addTemplateAndSettle(t, model, template)
	if reason := templateRowReason(t, model, template.ID); reason != `Already added as "`+template.Name+`"` {
		t.Fatalf("row reason = %q, want the already-added sentence", reason)
	}

	if result := executeLens(
		model, operationLensRename, LensEntity(template.ID), "Due, mine", false,
	); result.Err != nil {
		t.Fatalf("rename the added template: %v", result.Err)
	}
	renamed := model.runtime.Machine().NamedLensByID(template.ID)
	if renamed == nil {
		t.Fatal("the rename moved the record off its stable ID")
	}
	if renamed.Name != "Due, mine" {
		t.Fatalf("renamed record = %q, want the new name", renamed.Name)
	}
	if renamed.Mode != template.Mode || !reflect.DeepEqual(renamed.Tasks, template.Tasks) {
		t.Fatal("the rename rewrote the projection it was only supposed to relabel")
	}
	if reason := templateRowReason(t, model, template.ID); reason != `Already added as "Due, mine"` {
		t.Fatalf("row reason = %q, want the current label rather than the catalog one", reason)
	}

	// Replace overwrites the definition under the same ID with whatever the view
	// currently is, which is the furthest an added record can travel from its
	// template and still be it.
	out.selectOutlinerMode(ModeInbox)
	out.treeSort = SortState{Field: SortByTitle, Ascending: true}
	if result := executeLens(
		model, operationLensReplace, LensEntity(template.ID), "", true,
	); result.Err != nil {
		t.Fatalf("replace the added template: %v", result.Err)
	}
	replaced := model.runtime.Machine().NamedLensByID(template.ID)
	if replaced == nil {
		t.Fatal("the replace moved the record off its stable ID")
	}
	if replaced.Name != "Due, mine" {
		t.Fatalf("replace renamed the record to %q", replaced.Name)
	}
	if replaced.Mode == template.Mode && reflect.DeepEqual(replaced.Tasks, template.Tasks) {
		t.Fatal("the replace did not overwrite the definition with the current view")
	}
	if reason := templateRowReason(t, model, template.ID); reason != `Already added as "Due, mine"` {
		t.Fatalf("row reason = %q, want the replaced record still holding the ID", reason)
	}

	if result := executeLens(
		model, operationLensDelete, LensEntity(template.ID), "", true,
	); result.Err != nil {
		t.Fatalf("delete the added template: %v", result.Err)
	}
	if reason := templateRowReason(t, model, template.ID); reason != "" {
		t.Fatalf("row reason after delete = %q, want the template available again", reason)
	}

	// Re-adding reads the catalog, never the record that was deleted: the
	// definition that comes back is the original one, not the replaced one.
	addTemplateAndSettle(t, model, template)
	readded := model.runtime.Machine().NamedLensByID(template.ID)
	if readded == nil {
		t.Fatal("the re-add saved nothing under the template ID")
	}
	if !reflect.DeepEqual(*readded, template) {
		t.Fatalf("re-added record = %+v, want the original catalog definition %+v", *readded, template)
	}
}

// --- failure phases as the picker reports them -------------------------------

// lensPickerAppInMode opens the lens picker over a named mode, so a refusal can
// be asserted to have left the view underneath it alone.
func lensPickerAppInMode(t *testing.T, mode OutlinerModeID) *AppModel {
	t.Helper()
	model := lensOperationApp(t)
	model.crossOutliner.selectOutlinerMode(mode)
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}
	if model.lensPicker == nil {
		t.Fatalf("openNamedLensPicker left no picker mounted: %v", model.err)
	}
	return model
}

// TestAddTemplateFailuresRetainThePickerAndChangeNothing drives every way an add
// can be refused after the gallery has already drawn the row available. The
// gallery decides availability against the snapshot it renders, and the commit
// decides it again against the snapshot it writes; everything in this table
// happens in between. Each phase must leave the same four things: no gallery, a
// main picker carrying the reason, the durable list untouched, and the view the
// user was looking at unchanged.
func TestAddTemplateFailuresRetainThePickerAndChangeNothing(t *testing.T) {
	for _, phase := range []struct {
		name string
		// arm mutates the world after the gallery is open and the row is selected.
		arm func(t *testing.T, model *AppModel, template config.NamedLens)
		// want is the fragment the retained notice must carry.
		want string
		// rowReason is the gallery's verdict when the same row is revisited.
		rowReason string
	}{
		{
			name: "another lens claims the stable ID",
			arm: func(t *testing.T, model *AppModel, template config.NamedLens) {
				t.Helper()
				claimant := template.Clone()
				claimant.Name = "Claimed by hand"
				setMachineLenses(t, model, append(machineLenses(t, model).Clone(), claimant))
			},
			want:      `already added as "Claimed by hand"`,
			rowReason: `Already added as "Claimed by hand"`,
		},
		{
			name: "another lens claims the name",
			arm: func(t *testing.T, model *AppModel, template config.NamedLens) {
				t.Helper()
				stranger := template.Clone()
				stranger.ID = lensTemplateForeignID
				setMachineLenses(t, model, append(machineLenses(t, model).Clone(), stranger))
			},
			want:      "already exists",
			rowReason: `A lens named "` + "Overdue" + `" already exists`,
		},
		{
			name: "the Outliner is unavailable",
			arm: func(t *testing.T, model *AppModel, _ config.NamedLens) {
				t.Helper()
				model.crossOutliner = nil
			},
			want: "Outliner is unavailable",
			// Nothing durable moved, so the row is exactly as available as it was.
			rowReason: "",
		},
	} {
		t.Run(phase.name, func(t *testing.T) {
			model := lensPickerAppInMode(t, ModeInbox)
			template := addableTemplate(t, model)
			if template.Name != "Overdue" {
				t.Fatalf("the first addable template is %q; the name-collision row is written for Overdue", template.Name)
			}
			openAddTemplateGalleryFromPicker(t, model)
			selectGalleryRow(t, model, template.ID)
			phase.arm(t, model, template)
			before := lensNames(machineLenses(t, model))

			pressLens(t, model, "enter")

			if model.operationValuePicker != nil {
				t.Error("a refused add left the gallery mounted")
			}
			if model.lensPicker == nil {
				t.Fatal("a refused add closed the main picker")
			}
			if notice := model.lensPicker.Notice(); !strings.Contains(notice, phase.want) {
				t.Errorf("picker notice = %q, want it to name %q", notice, phase.want)
			}
			if got := lensNames(machineLenses(t, model)); !slices.Equal(got, before) {
				t.Errorf("durable list = %v, want the untouched %v", got, before)
			}
			if out := model.crossOutliner; out != nil {
				if out.hasActiveLens() {
					t.Error("a refused add applied a lens")
				}
				if out.pendingLens != nil {
					t.Error("a refused add left a population load armed")
				}
				if out.primaryMode != ModeInbox {
					t.Errorf("current view = %v, want the untouched %v", out.primaryMode, ModeInbox)
				}
			}

			// The reason is retained on the surface that raised it, and the gallery
			// the same row reopens now reports the state the commit refused against.
			pressLens(t, model, "enter")
			if model.operationValuePicker == nil {
				t.Fatal("retrying the add raised no gallery")
			}
			if got := galleryRow(t, model.operationValuePicker, template.ID).DisabledReason; got != phase.rowReason {
				t.Errorf("revisited row reason = %q, want %q", got, phase.rowReason)
			}
		})
	}
}

// TestAddTemplateDiskCommitFailureRetainsThePicker is the durable half of the
// same rule. The commit is what publishes the record and the application runs
// only after it returns, so a write that cannot land leaves the machine, the
// cached snapshot, and the current view exactly as they were — and says so on the
// picker rather than anywhere else.
func TestAddTemplateDiskCommitFailureRetainsThePicker(t *testing.T) {
	model := lensPickerAppInMode(t, ModeInbox)
	dataDir := t.TempDir()
	project := models.Project{ID: "alpha", Name: "alpha", Slug: "alpha"}
	model.runtime = testRuntimeAt(dataDir, testConfigWithProjects(&project))
	template := addableTemplate(t, model)
	openAddTemplateGalleryFromPicker(t, model)
	selectGalleryRow(t, model, template.ID)

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Chmod(home, 0o500); err != nil {
		t.Skipf("cannot make the runtime root read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	before := model.runtime.Machine()
	pressLens(t, model, "enter")

	if model.runtime.Machine() != before {
		t.Skip("the runtime root stayed writable; the failure path is untested here")
	}
	if model.operationValuePicker != nil {
		t.Error("a failed commit left the gallery mounted")
	}
	if model.lensPicker == nil {
		t.Fatal("a failed commit closed the main picker")
	}
	if model.lensPicker.Notice() == "" {
		t.Error("a failed commit reported nothing on the picker")
	}
	if before.NamedLensByID(template.ID) != nil {
		t.Error("a failed commit added the template to the cached snapshot")
	}
	if out := model.crossOutliner; out.hasActiveLens() || out.primaryMode != ModeInbox {
		t.Error("a failed commit applied the template anyway")
	}
}

// --- Parked Work through the asynchronous population path --------------------

// parkedTemplateApp builds the population Parked Work is read against: one active
// project the base task workspace holds, two parked ones it never does, and a
// parked project that is also archived. Only the lens's own facets can reach past
// the base workspace, and archival is authoritative over all of them.
func parkedTemplateApp(t *testing.T) (*AppModel, *OutlinerModel, []*models.Project) {
	t.Helper()
	alpha := primaryRowsProject("alpha", primaryRowsTask("alpha-open", models.StateBacklog))
	parkedOne := primaryRowsProject("parked-one", primaryRowsTask("parked-one-open", models.StateBacklog))
	parkedOne.Tags = []string{models.ParkedProjectTag}
	parkedTwo := primaryRowsProject("parked-two", primaryRowsTask("parked-two-open", models.StateBacklog))
	parkedTwo.Tags = []string{models.ParkedProjectTag}
	archived := primaryRowsProject("archived-parked", primaryRowsTask("archived-open", models.StateBacklog))
	archived.Tags = []string{models.ParkedProjectTag}
	archived.Archived = true

	model := lensOperationApp(t)
	out := scopeTestOutliner(alpha, parkedOne, parkedTwo, archived)
	out.loadedProjects = []*models.Project{alpha}
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	model.crossOutliner = out
	out.SetSize(model.width, model.height-2)
	model.views.Register(ViewCrossProjectOutliner, out)
	return model, out, []*models.Project{alpha, parkedOne, parkedTwo, archived}
}

func parkedWorkTemplate(t *testing.T) config.NamedLens {
	t.Helper()
	return lensTemplateByName(t, "Parked Work")
}

// TestParkedWorkCommitsBeforeItsPopulationLoads pins the ordering the add handler
// depends on: the record is durable before a byte of the population is read, and
// the lens itself publishes only when its own load lands.
func TestParkedWorkCommitsBeforeItsPopulationLoads(t *testing.T) {
	model, out, population := parkedTemplateApp(t)
	template := parkedWorkTemplate(t)

	result := executeLensValue(model, template.ID)
	if result.Err != nil {
		t.Fatalf("add Parked Work: %v", result.Err)
	}

	// Committed already.
	saved := model.runtime.Machine().NamedLensByID(template.ID)
	if saved == nil {
		t.Fatal("the record was not committed before the load started")
	}
	if !reflect.DeepEqual(*saved, template) {
		t.Fatalf("saved record = %+v, want the catalog definition %+v", *saved, template)
	}
	// Applied not at all.
	if out.pendingLens == nil {
		t.Fatal("adding Parked Work started no population load")
	}
	if out.hasActiveLens() {
		t.Fatal("the lens published before its population landed")
	}
	if ids := loadedProjectIDs(out); !slices.Equal(ids, []string{"alpha"}) {
		t.Fatalf("population before the load landed = %v, want the untouched [alpha]", ids)
	}

	settleLensPopulationLoad(t, out, population...)

	if !out.hasActiveLens() || out.activeLens.lensID != template.ID {
		t.Fatal("the landed load did not publish the lens")
	}
	// Parked and active: the archived parked project is not a member of any
	// population the base workspace can select.
	if ids := loadedProjectIDs(out); !slices.Equal(ids, []string{"parked-one", "parked-two"}) {
		t.Fatalf("population = %v, want the parked, non-archived projects", ids)
	}
	if ids := taskRowProjectIDs(out); !slices.Equal(ids, []string{"parked-one", "parked-two"}) {
		t.Fatalf("rows = %v, want the parked, non-archived projects", ids)
	}
	if out.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("a landed load left the population facets still owed")
	}
	if out.crossLoading {
		t.Fatal("the landed load left the loading state armed")
	}
}

// TestParkedWorkLoadFailuresKeepTheirExistingGuarantees confirms the two
// load-failure rules still hold on the route a catalog template takes: a wholly
// failed read abandons the application rather than installing a lens over an
// empty population, and a partial one publishes what read and reports what did
// not.
func TestParkedWorkLoadFailuresKeepTheirExistingGuarantees(t *testing.T) {
	t.Run("total failure abandons the application", func(t *testing.T) {
		model, out, _ := parkedTemplateApp(t)
		template := parkedWorkTemplate(t)
		baseline := loadedProjectIDs(out)

		if result := executeLensValue(model, template.ID); result.Err != nil {
			t.Fatalf("add Parked Work: %v", result.Err)
		}
		cmd, published := out.handleLensProjectsLoaded(lensProjectsLoadedMsg{
			failedLoads: []projectLoadFailure{
				{ProjectID: "parked-one", Detail: "failed to load project parked-one: boom"},
			},
			generation: out.pendingLens.generation,
		})
		if published {
			t.Fatal("a wholly failed load published the lens")
		}
		if out.hasActiveLens() {
			t.Fatal("a wholly failed load installed the lens")
		}
		if ids := loadedProjectIDs(out); !slices.Equal(ids, baseline) {
			t.Fatalf("population = %v, want the untouched %v", ids, baseline)
		}
		if cmd == nil {
			t.Fatal("a wholly failed load must report why")
		}
		if _, ok := cmd().(ErrorMsg); !ok {
			t.Fatal("a wholly failed load must raise an error")
		}
		// The record is durable either way: the commit succeeded, and only the
		// application was abandoned.
		if model.runtime.Machine().NamedLensByID(template.ID) == nil {
			t.Fatal("the abandoned application rolled back a commit that had succeeded")
		}
	})

	t.Run("partial failure publishes what read", func(t *testing.T) {
		model, out, _ := parkedTemplateApp(t)
		template := parkedWorkTemplate(t)

		if result := executeLensValue(model, template.ID); result.Err != nil {
			t.Fatalf("add Parked Work: %v", result.Err)
		}
		cmd, published := out.handleLensProjectsLoaded(lensProjectsLoadedMsg{
			projects: []*models.Project{
				primaryRowsProject("parked-one", primaryRowsTask("parked-one-open", models.StateBacklog)),
			},
			failedLoads: []projectLoadFailure{
				{ProjectID: "parked-two", Detail: "failed to load project parked-two: boom"},
			},
			generation: out.pendingLens.generation,
		})
		if !published {
			t.Fatal("a partial load must publish the projects that read")
		}
		if !out.hasActiveLens() || out.activeLens.lensID != template.ID {
			t.Fatal("a partial load did not install the lens")
		}
		if ids := loadedProjectIDs(out); !slices.Equal(ids, []string{"parked-one"}) {
			t.Fatalf("population = %v, want the project that read", ids)
		}
		if cmd == nil {
			t.Fatal("a partial load must report the projects it dropped")
		}
		if _, ok := cmd().(InfoStatusMsg); !ok {
			t.Fatal("a partial load must warn rather than error")
		}
	})
}

// TestParkedWorkFacetsStayInertUnderScopeAndSelectOnExit is the scoped half of
// the same template. A project scope has already fixed the population to one
// project, so the facets Parked Work carries are recorded and decide nothing;
// leaving the scope is the moment they choose a population.
func TestParkedWorkFacetsStayInertUnderScopeAndSelectOnExit(t *testing.T) {
	model, out, population := parkedTemplateApp(t)
	template := parkedWorkTemplate(t)
	out.pushProjectScope("alpha")

	if result := executeLensValue(model, template.ID); result.Err != nil {
		t.Fatalf("add Parked Work under a scope: %v", result.Err)
	}
	if out.pendingLens != nil {
		t.Fatal("a scoped application must record its facets without loading")
	}
	if !out.hasActiveLens() {
		t.Fatal("a scoped application did not publish the lens it needed no load for")
	}
	frame := out.activeLens
	if !frame.projectFacetsAwaitingLoad {
		t.Fatal("the scoped application recorded no owed population load")
	}
	if out.projectFilter.ParkedMode != projectPopulationOnly {
		t.Fatal("the scoped application did not record the template's population facets")
	}
	if ids := taskRowProjectIDs(out); !slices.Equal(ids, []string{"alpha"}) {
		t.Fatalf("scoped rows = %v, want the scope's own [alpha]", ids)
	}

	handled, cmd := model.applyProjectScope(ApplyProjectScopeMsg{Clear: true})
	if !handled || cmd == nil {
		t.Fatal("clearing the scope under Parked Work started no population load")
	}
	if out.pendingLens == nil {
		t.Fatal("the exit armed no pending application")
	}
	settleLensPopulationLoad(t, out, population...)

	if out.isProjectScoped() {
		t.Fatal("the settled exit left the scope in place")
	}
	if ids := loadedProjectIDs(out); !slices.Equal(ids, []string{"parked-one", "parked-two"}) {
		t.Fatalf("population after the exit = %v, want the parked, non-archived projects", ids)
	}
	// The exit publishes a population, never a definition.
	if out.activeLens != frame {
		t.Fatal("the exit replaced the frame the scoped application installed")
	}
	if out.lensModified() {
		t.Fatal("the exit marked the lens modified")
	}
}

// --- the narrowing templates' own semantics ----------------------------------

func lensSemanticsTask(id string, state models.WorkflowState) *models.Task {
	task := primaryRowsTask(id, state)
	task.CreatedAt = time.Now()
	return task
}

// daysFromNow is a calendar date, midnight local: a due date carrying a
// time-of-day is compared at its exact instant instead of at day granularity,
// and "due today" would then read as already overdue by however long today has
// been running.
func daysFromNow(days int) *time.Time {
	when := util.StartOfDay(time.Now().AddDate(0, 0, days))
	return &when
}

// lensSemanticsOutliner holds the one population the narrowing templates are read
// against: dated work either side of today, tasks carrying the review vocabulary
// in both the tag and the context axis, and terminal work either side of the
// ninety-day completion threshold.
func lensSemanticsOutliner() *OutlinerModel {
	overdueNear := lensSemanticsTask("overdue-near", models.StateBacklog)
	overdueNear.DueDate = daysFromNow(-1)
	overdueFar := lensSemanticsTask("overdue-far", models.StateBacklog)
	overdueFar.DueDate = daysFromNow(-9)
	dueToday := lensSemanticsTask("due-today", models.StateBacklog)
	dueToday.DueDate = daysFromNow(0)
	dueLater := lensSemanticsTask("due-later", models.StateBacklog)
	dueLater.DueDate = daysFromNow(3)
	overdueDone := lensSemanticsTask("overdue-done", models.StateDone)
	overdueDone.DueDate = daysFromNow(-4)
	overdueDone.CompletedAt = daysFromNow(-1)

	reviewTagged := lensSemanticsTask("review-tagged", models.StateBacklog)
	reviewTagged.Tags = []string{"review"}
	reviewContext := lensSemanticsTask("review-context", models.StateBacklog)
	reviewContext.Context = "review"
	reviewUrgent := lensSemanticsTask("review-urgent", models.StateBacklog)
	reviewUrgent.Tags = []string{"review", "urgent"}
	urgentOnly := lensSemanticsTask("urgent-only", models.StateBacklog)
	urgentOnly.Tags = []string{"urgent"}

	doneOld := lensSemanticsTask("done-old", models.StateDone)
	doneOld.CompletedAt = daysFromNow(-120)
	canceledOld := lensSemanticsTask("canceled-old", models.StateCanceled)
	canceledOld.CompletedAt = daysFromNow(-200)
	doneRecent := lensSemanticsTask("done-recent", models.StateDone)
	doneRecent.CompletedAt = daysFromNow(-10)
	// A nonterminal task wearing an old completion stamp is deliberately
	// impossible-looking: it is the one shape that isolates Cleanup 90d's state
	// predicate from its age predicate, because only the state predicate can
	// reject it.
	waitingOld := lensSemanticsTask("waiting-old", models.StateWaiting)
	waitingOld.CompletedAt = daysFromNow(-200)

	return scopeTestOutliner(primaryRowsProject(
		"alpha",
		overdueNear, overdueFar, dueToday, dueLater, overdueDone,
		reviewTagged, reviewContext, reviewUrgent, urgentOnly,
		doneOld, canceledOld, doneRecent, waitingOld,
	))
}

func sortedVisibleTaskIDs(outliner *OutlinerModel) []string {
	ids := slices.Clone(searchVisibleTaskIDs(outliner))
	sort.Strings(ids)
	return ids
}

// TestOverdueTemplateUsesDueModeFixedOrdering pins Overdue to the ordering its
// mode owns. The template narrows with the overdue bucket and nothing else; the
// order the rows arrive in is Due mode's calendar order, not the manual sort the
// definition still carries for whenever a tree-order mode reads it.
func TestOverdueTemplateUsesDueModeFixedOrdering(t *testing.T) {
	outliner := lensSemanticsOutliner()
	template := lensTemplateByName(t, "Overdue")
	applyLensOrFatal(t, outliner, template)

	mode, ok := OutlinerModeByID(outliner.primaryMode)
	if !ok || outliner.primaryMode != ModeDue {
		t.Fatalf("mode = %v, want Due", outliner.primaryMode)
	}
	if !mode.FixedOrder() {
		t.Fatal("Due must own a fixed order for the template to inherit one")
	}
	if outliner.sort != NewSortState() {
		t.Fatalf("active sort = %+v, want a fixed-order mode's inactive default", outliner.sort)
	}

	// Manual order in the project puts overdue-near first; the calendar order the
	// mode imposes puts the earlier due date first instead. Terminal work is not
	// overdue however far past its date it sits.
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"overdue-far", "overdue-near"}) {
		t.Fatalf("rows = %v, want the overdue bucket in due-date order", got)
	}
}

// TestReviewQueueTemplateIntersectsItsTagFacet pins Review Queue to the tag
// semantics the facet has everywhere else: selected tags are ANDed, so adding one
// narrows rather than broadens. The single tag the template ships with matches
// either tag vocabulary, which is the facet's own rule and not a template
// exception.
func TestReviewQueueTemplateIntersectsItsTagFacet(t *testing.T) {
	outliner := lensSemanticsOutliner()
	applyLensOrFatal(t, outliner, lensTemplateByName(t, "Review Queue"))

	want := []string{"review-context", "review-tagged", "review-urgent"}
	if got := sortedVisibleTaskIDs(outliner); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want every task carrying the review vocabulary %v", got, want)
	}

	// A second tag intersects with the first rather than joining it.
	outliner.filter.Tags["urgent"] = true
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if got := sortedVisibleTaskIDs(outliner); !slices.Equal(got, []string{"review-urgent"}) {
		t.Fatalf("rows under two tags = %v, want only the task carrying both", got)
	}
}

// TestCleanup90dTemplateCombinesItsTwoPredicates pins Cleanup 90d to the
// conjunction its two facets make. Terminal state and completion age are separate
// predicates that both have to hold; neither is widened to carry the other.
func TestCleanup90dTemplateCombinesItsTwoPredicates(t *testing.T) {
	outliner := lensSemanticsOutliner()
	applyLensOrFatal(t, outliner, lensTemplateByName(t, "Cleanup 90d"))

	want := []string{"canceled-old", "done-old"}
	if got := sortedVisibleTaskIDs(outliner); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want the terminal work older than the threshold %v", got, want)
	}
	// done-recent is terminal but young, and waiting-old is old but not terminal.
	// Their absence above is the whole assertion; naming them here keeps the
	// failure message honest about which predicate did the rejecting.
	for _, rejected := range []string{"done-recent", "waiting-old", "overdue-done"} {
		if slices.Contains(searchVisibleTaskIDs(outliner), rejected) {
			t.Errorf("Cleanup 90d admitted %q, so one of its two predicates was broadened", rejected)
		}
	}
}
