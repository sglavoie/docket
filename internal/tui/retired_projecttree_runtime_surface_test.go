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

	t.Run("focus queue", func(t *testing.T) {
		fq := NewProjectFocusViewModel(project, runtime)
		fq.SetSize(120, 30)
		// No queued tasks → the empty-state hint renders; it must not tell the
		// user to press q "in the project tree".
		assertClean(t, "focus queue empty-state view", stripANSI(fq.View().Content))
		for _, item := range helpItemsFromKeyMap(fq.buildKeyMap()) {
			assertClean(t, "focus queue keymap", item.Description)
		}
	})

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

	t.Run("all tasks", func(t *testing.T) {
		at := NewAllTasksViewModel(runtime, mgr)
		for _, h := range AllTasksNavigationPrefixHints {
			assertClean(t, "all tasks nav prefix hint", h.Description+" "+h.Detail)
		}
		for _, item := range helpItemsFromKeyMap(at.buildKeyMap()) {
			assertClean(t, "all tasks keymap", item.Description)
		}
	})

	t.Run("priority", func(t *testing.T) {
		pv := NewPriorityViewModel(project, runtime, mgr)
		for _, item := range helpItemsFromKeyMap(pv.buildKeyMap()) {
			assertClean(t, "priority keymap", item.Description)
		}
	})

	t.Run("roadmap", func(t *testing.T) {
		rm := NewRoadmapModel(runtime)
		for _, item := range helpItemsFromKeyMap(rm.buildKeyMap()) {
			assertClean(t, "roadmap keymap", item.Description)
		}
	})

	t.Run("pipeline hub work column exposes one outliner route", func(t *testing.T) {
		hub := NewPipelineHubModel(runtime, nil, nil, project)
		var work pipelineColumn
		for _, col := range hub.columns {
			if col.name == pipelineWorkColumnName {
				work = col
				break
			}
		}
		if work.name == "" {
			t.Fatal("pipeline hub missing Work column")
		}
		outlinerRoutes := 0
		for _, e := range work.entries {
			assertClean(t, "pipeline hub work entry", e.label)
			if e.viewType == ViewOutliner {
				outlinerRoutes++
			}
		}
		if outlinerRoutes != 1 {
			t.Fatalf("Work column should expose exactly one route to the current project's Outliner, got %d", outlinerRoutes)
		}
	})
}
