package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// TestRetiredProjectTreeRuntimeSurfacesRemoved is the completion contract for
// the retired-projecttree-runtime-surface repair (FR-010-004). Session 9.3's
// TestRetiredProjectTreeSurfacesRemoved froze settings/tutorial/palette-entry
// state; this test scans the *live* runtime help, keymap descriptions, and
// rendered strings of every per-project surface that used to send users back to
// the deleted project tree, plus the Pipeline Hub's duplicate Work route.
func TestRetiredProjectTreeRuntimeSurfacesRemoved(t *testing.T) {
	assertClean := func(t *testing.T, surface, text string) {
		t.Helper()
		low := strings.ToLower(text)
		if strings.Contains(low, "project tree") || strings.Contains(low, "in tree") {
			t.Errorf("%s still references the retired project tree: %q", surface, text)
		}
	}

	cfg, project := newTestConfigWithProject("Runtime Surfaces")
	runtime := testRuntimeAt(cfg.DataDir(), cfg)
	mgr := config.NewManager(config.ConfigPathForDataDir(t.TempDir()))

	t.Run("inbox", func(t *testing.T) {
		inbox := NewInboxProcessorModel(project, mgr, runtime)
		assertClean(t, "inbox returnHint", inbox.returnHint())
		for _, item := range helpItemsFromKeyMap(inbox.buildKeyMap()) {
			assertClean(t, "inbox keymap", item.Description)
		}
	})

	t.Run("palette", func(t *testing.T) {
		app := newTestAppModel()
		for _, group := range app.commandGroups() {
			for _, cmd := range group {
				hay := cmd.Name + " " + cmd.Description + " " + strings.Join(cmd.Aliases, " ")
				assertClean(t, "palette command "+cmd.Name, hay)
			}
		}
	})

	t.Run("roadmap", func(t *testing.T) {
		rm := NewRoadmapModel(runtime)
		for _, item := range helpItemsFromKeyMap(rm.buildKeyMap()) {
			assertClean(t, "roadmap keymap", item.Description)
		}
	})

}
