package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestRetiredProjectFilterSurfacesRemoved is the completion contract for the
// Context Hub redesign's removal of the ad-hoc project-subset filter (F P). The
// active Context is now the single project-set authority, so no TUI path may
// expose an arbitrary unsaved project subset: the keymap must not bind F P, the
// runtime F popup must not advertise it, no help/keymap/palette text may promise
// a project filter, the ProjectFilterSelector presentation must be gone, and the
// filter badge must never emit a proj: count.
func TestRetiredProjectFilterSurfacesRemoved(t *testing.T) {
	t.Run("no outliner binds F P", func(t *testing.T) {
		singleProject := (&OutlinerModel{project: &models.Project{}}).buildKeyMap().HandledKeys()
		if containsKey(singleProject, "FP") {
			t.Fatalf("single-project outliner still binds retired F P: %v", singleProject)
		}
		crossProject := (&OutlinerModel{}).buildKeyMap().HandledKeys()
		if containsKey(crossProject, "FP") {
			t.Fatalf("cross-project outliner still binds retired F P: %v", crossProject)
		}
	})

	t.Run("runtime F popup omits the P hint", func(t *testing.T) {
		for _, view := range []string{"Outliner", "CrossProjectOutliner"} {
			info := runtimeFilterPopupInfo(t, view)
			if info == nil {
				t.Fatalf("%s produced no F prefix popup", view)
			}
			for _, hint := range info.Hints["F"] {
				if hint.Key == "P" {
					t.Fatalf("%s F popup still advertises the retired project filter: %+v", view, hint)
				}
			}
		}
	})

	t.Run("no outliner help text promises a project filter", func(t *testing.T) {
		for _, view := range []PresentationID{ViewOutliner, ViewCrossProjectOutliner} {
			for _, item := range GetKeybindingRegistry().GetForView(view) {
				assertNoProjectFilterWording(t, "registry "+viewTypeNames[view], item.Description)
			}
		}
		for _, km := range []*OutlinerModel{{project: &models.Project{}}, {}} {
			for _, item := range helpItemsFromKeyMap(km.buildKeyMap()) {
				assertNoProjectFilterWording(t, "outliner keymap", item.Description)
			}
		}
	})

	t.Run("palette drops project-filter commands", func(t *testing.T) {
		app := newTestAppModel()
		for _, group := range app.commandGroups() {
			for _, cmd := range group {
				assertNoProjectFilterWording(t, "palette command "+cmd.Name,
					cmd.Name+" "+cmd.Description+" "+strings.Join(cmd.Aliases, " "))
			}
		}
	})

	t.Run("context hub help never promises a project filter", func(t *testing.T) {
		app := contextHubApp(t)
		hub := openContextHub(t, app)
		for _, item := range hub.GetHelpItems() {
			assertNoProjectFilterWording(t, "context hub help", item.Description)
		}
		hub.Update(syntheticKey('n'))
		if hub.editor == nil {
			t.Fatal("n did not open the inline context editor")
		}
		for _, item := range hub.GetHelpItems() {
			assertNoProjectFilterWording(t, "context editor help", item.Description)
		}
	})

	t.Run("no ProjectFilterSelector presentation remains", func(t *testing.T) {
		for id, name := range viewTypeNames {
			if strings.Contains(strings.ToLower(name), "projectfilter") {
				t.Fatalf("presentation %d still registered as %q", id, name)
			}
		}
	})

	t.Run("filter badge never reports a project subset", func(t *testing.T) {
		m := &OutlinerModel{filter: NewFilterState()}
		m.filter.Priority = 1
		for _, part := range m.activeFilterParts() {
			if strings.Contains(part, "proj:") {
				t.Fatalf("filter badge still reports a project subset: %q", part)
			}
		}
	})
}

func assertNoProjectFilterWording(t *testing.T, surface, text string) {
	t.Helper()
	low := strings.ToLower(text)
	for _, phrase := range []string{"filter by project", "project filter", "filter by projects", "filter projects"} {
		if strings.Contains(low, phrase) {
			t.Errorf("%s still advertises the retired project filter: %q", surface, text)
			return
		}
	}
}
