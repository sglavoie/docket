package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// minimalFactoryConfig is the smallest config the primary view constructors
// accept: a single current project (the project-scoped views require one) with
// the maps they read initialized.
func minimalFactoryConfig(t *testing.T) *config.RuntimeState {
	t.Helper()
	project, err := models.NewProject("proj-1", "Factory Project", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	runtime := testConfigWithProjects(project)
	runtime.Machine().CurrentProject = project.ID
	runtime.Machine().ProjectViewStates = make(map[string]config.ProjectViewState)
	runtime.Settings().SectionColors = make(map[string]string)
	return runtime
}

// TestPrimaryFeatureFactoriesPopulated asserts every primary feature view that
// primaryFeatureFactories registers has a non-nil Factory in the built route
// table, so the router can construct it. Views outside the map keep a nil
// Factory (still built by their legacy call sites) and are not asserted here.
func TestPrimaryFeatureFactoriesPopulated(t *testing.T) {
	for vt := range primaryFeatureFactories() {
		if routeFor(vt).Factory == nil {
			t.Errorf("route %s has a nil Factory; expected it to be populated", vt)
		}
	}
}

// TestPrimaryFeatureFactoriesBuildViews invokes each primary factory with a
// minimal ViewContext and asserts it returns a non-nil View. This guards the
// factory wiring (correct constructor, View conformance) without navigating.
func TestPrimaryFeatureFactoriesBuildViews(t *testing.T) {
	cfg := minimalFactoryConfig(t)
	ctx := ViewContext{Width: 80, Height: 24, Runtime: testRuntimeAt(cfg.DataDir(), cfg)}
	for vt, factory := range primaryFeatureFactories() {
		view := factory(ctx, RouteParams{})
		if view == nil {
			t.Errorf("factory for %s returned a nil View", vt)
			continue
		}
		// A View must accept sizing without panicking.
		view.SetSize(80, 24)
	}
}

// TestNonPrimaryRoutesKeepNilFactory confirms populating the primary factories
// did not accidentally assign factories to unrelated routes.
func TestNonPrimaryRoutesKeepNilFactory(t *testing.T) {
	primary := primaryFeatureFactories()
	for _, vt := range allViewTypes() {
		if _, isPrimary := primary[vt]; isPrimary {
			continue
		}
		if routeFor(vt).Factory != nil {
			t.Errorf("non-primary route %s unexpectedly has a Factory", vt)
		}
	}
}
