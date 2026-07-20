package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TestRetiredProjectTreeSurfacesRemoved is the completion contract for the
// retired-projecttree-surface repair (FR-009-004). The Outliner is the sole
// surviving per-project view; the ProjectTree must no longer appear as a
// selectable setting, a dead column-config tab, a no-op ctrl+p promise, or a
// tutorial/command-palette surface.
func TestRetiredProjectTreeSurfacesRemoved(t *testing.T) {
	t.Run("settings drop the Project Tree tab", func(t *testing.T) {
		for _, name := range settingsTabNames {
			if strings.EqualFold(name, "Project Tree") {
				t.Fatalf("settings still expose a %q tab", name)
			}
		}
		// The three index-driven registries must stay aligned after the removal.
		if len(settingsTabNames) != len(tabFieldCount) || len(settingsTabNames) != len(settingsTabSections) {
			t.Fatalf("settings registries misaligned: names=%d counts=%d sections=%d",
				len(settingsTabNames), len(tabFieldCount), len(settingsTabSections))
		}
		if tabAppearance != len(settingsTabNames)-1 {
			t.Fatalf("Appearance tab index = %d, want compacted final index %d", tabAppearance, len(settingsTabNames)-1)
		}
	})

	t.Run("no selectable Default View / project-tree field", func(t *testing.T) {
		for _, f := range settingsFields {
			hay := strings.ToLower(f.TabLabel + " " + f.Label + " " + f.Keywords)
			if strings.Contains(hay, "project tree") {
				t.Fatalf("settings field still references project tree: %+v", f)
			}
			if strings.EqualFold(f.Label, "Default View") {
				t.Fatalf("settings still expose a selectable %q field", f.Label)
			}
		}
	})

	t.Run("legacy project_tree preference migrates to outliner", func(t *testing.T) {
		s := &config.Settings{DefaultProjectView: "project_tree"}
		if got := s.GetDefaultProjectView(); got != "outliner" {
			t.Fatalf("GetDefaultProjectView() = %q, want outliner (safe migration)", got)
		}
		// The navigation resolver always lands on the Outliner regardless of value.
		if got := defaultProjectViewType(s); got != ViewOutliner {
			t.Fatalf("defaultProjectViewType() = %v, want ViewOutliner", got)
		}
	})

	t.Run("ctrl+p is cross-project drill-down only", func(t *testing.T) {
		singleProject := (&OutlinerModel{project: &models.Project{}}).buildKeyMap().HandledKeys()
		if containsKey(singleProject, "ctrl+p") {
			t.Fatalf("single-project outliner still binds no-op ctrl+p: %v", singleProject)
		}
		crossProject := (&OutlinerModel{}).buildKeyMap().HandledKeys()
		if !containsKey(crossProject, "ctrl+p") {
			t.Fatalf("cross-project outliner dropped its meaningful ctrl+p drill-down: %v", crossProject)
		}
	})

	t.Run("tutorials no longer promise the project tree", func(t *testing.T) {
		outliner := findTutorTUITopic(t, "Outliner")
		if strings.Contains(strings.ToLower(outliner.Content), "project tree") {
			t.Fatalf("Outliner tutorial still promises the project tree:\n%s", outliner.Content)
		}
		for _, topic := range tutorTopics() {
			if strings.Contains(strings.ToLower(topic.Content), "project tree") {
				t.Fatalf("tutorial %q still promises the retired project tree:\n%s", topic.Title, topic.Content)
			}
		}
	})

	t.Run("command palette drops project-tree wording", func(t *testing.T) {
		app := newTestAppModel()
		for _, cmd := range app.viewCommands() {
			hay := strings.ToLower(cmd.Name + " " + cmd.Description + " " + strings.Join(cmd.Aliases, " "))
			if strings.Contains(hay, "tree") {
				t.Fatalf("view command still advertises a tree surface: %+v", cmd)
			}
		}
		// The meaningful cross-project drill-down command survives with outliner wording.
		if cmd := findCommand(app.viewCommands(), "open project outliner"); cmd == nil {
			t.Fatal("expected 'open project outliner' cross-project drill-down command")
		}
	})
}

func containsKey(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}
