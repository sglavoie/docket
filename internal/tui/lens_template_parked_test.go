package tui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
)

// parkedTemplateApp builds the population Parked Work is read against: one active
// project the base task workspace holds, two parked ones it never does, and a
// parked project that is also archived. Only the lens's own facets can reach past
// the base workspace, and archival is authoritative over all of them.
func parkedTemplateApp(t *testing.T) (*AppModel, *outliner.OutlinerModel, []*models.Project) {
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
	out.Population.Projects = []*models.Project{alpha}
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	model.crossOutliner = out
	out.SetSize(model.viewNav.viewSize())
	model.viewNav.views.Register(presentation.ViewCrossProjectOutliner, out)
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
	if out.Lens.Pending == nil {
		t.Fatal("adding Parked Work started no population load")
	}
	if out.HasActiveLens() {
		t.Fatal("the lens published before its population landed")
	}
	if ids := loadedProjectIDs(out); !slices.Equal(ids, []string{"alpha"}) {
		t.Fatalf("population before the load landed = %v, want the untouched [alpha]", ids)
	}

	settleLensPopulationLoad(t, out, population...)

	if !out.HasActiveLens() || out.Lens.Active.LensID != template.ID {
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
	if out.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("a landed load left the population facets still owed")
	}
	if out.Population.Loading {
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
		cmd, published := out.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
			FailedLoads: []outliner.ProjectLoadFailure{
				{ProjectID: "parked-one", Detail: "failed to load project parked-one: boom"},
			},
			Generation: out.Lens.Pending.Generation,
		})
		if published {
			t.Fatal("a wholly failed load published the lens")
		}
		if out.HasActiveLens() {
			t.Fatal("a wholly failed load installed the lens")
		}
		if ids := loadedProjectIDs(out); !slices.Equal(ids, baseline) {
			t.Fatalf("population = %v, want the untouched %v", ids, baseline)
		}
		if cmd == nil {
			t.Fatal("a wholly failed load must report why")
		}
		if _, ok := cmd().(status.ErrorMsg); !ok {
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
		cmd, published := out.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
			Projects: []*models.Project{
				primaryRowsProject("parked-one", primaryRowsTask("parked-one-open", models.StateBacklog)),
			},
			FailedLoads: []outliner.ProjectLoadFailure{
				{ProjectID: "parked-two", Detail: "failed to load project parked-two: boom"},
			},
			Generation: out.Lens.Pending.Generation,
		})
		if !published {
			t.Fatal("a partial load must publish the projects that read")
		}
		if !out.HasActiveLens() || out.Lens.Active.LensID != template.ID {
			t.Fatal("a partial load did not install the lens")
		}
		if ids := loadedProjectIDs(out); !slices.Equal(ids, []string{"parked-one"}) {
			t.Fatalf("population = %v, want the project that read", ids)
		}
		if cmd == nil {
			t.Fatal("a partial load must report the projects it dropped")
		}
		if _, ok := cmd().(status.InfoStatusMsg); !ok {
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
	out.PushProjectScope("alpha")

	if result := executeLensValue(model, template.ID); result.Err != nil {
		t.Fatalf("add Parked Work under a scope: %v", result.Err)
	}
	if out.Lens.Pending != nil {
		t.Fatal("a scoped application must record its facets without loading")
	}
	if !out.HasActiveLens() {
		t.Fatal("a scoped application did not publish the lens it needed no load for")
	}
	frame := out.Lens.Active
	if !frame.ProjectFacetsAwaitingLoad {
		t.Fatal("the scoped application recorded no owed population load")
	}
	if out.ProjectFilter.ParkedMode != query.ProjectPopulationOnly {
		t.Fatal("the scoped application did not record the template's population facets")
	}
	if ids := taskRowProjectIDs(out); !slices.Equal(ids, []string{"alpha"}) {
		t.Fatalf("scoped rows = %v, want the scope's own [alpha]", ids)
	}

	handled, cmd := model.applyProjectScope(ApplyProjectScopeMsg{Clear: true})
	if !handled || cmd == nil {
		t.Fatal("clearing the scope under Parked Work started no population load")
	}
	if out.Lens.Pending == nil {
		t.Fatal("the exit armed no pending application")
	}
	settleLensPopulationLoad(t, out, population...)

	if out.IsProjectScoped() {
		t.Fatal("the settled exit left the scope in place")
	}
	if ids := loadedProjectIDs(out); !slices.Equal(ids, []string{"parked-one", "parked-two"}) {
		t.Fatalf("population after the exit = %v, want the parked, non-archived projects", ids)
	}
	// The exit publishes a population, never a definition.
	if out.Lens.Active != frame {
		t.Fatal("the exit replaced the frame the scoped application installed")
	}
	// Scope is lens-owned, so dropping the one the template was applied under is a
	// divergence the marker reports. The definition itself is untouched: the frame
	// above is the same one, saved fingerprint and all.
	if !out.LensModified() {
		t.Fatal("leaving a project scope under a live lens must mark it modified")
	}
}
