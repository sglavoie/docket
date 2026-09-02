package tui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
		ProjectFilter:  pending.state.projectFilterState(),
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

// --- Parked Work through the asynchronous population path --------------------
