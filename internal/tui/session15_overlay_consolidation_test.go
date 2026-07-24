package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestSession15EmptyPaletteIsBoundedToRecentAndContextualCommands(t *testing.T) {
	app := newTestAppModel()
	registry := NewCommandRegistry()
	for i := range 16 {
		name := fmt.Sprintf("session15 command %02d", i)
		if err := registry.Register(Command{Name: name, Description: name}); err != nil {
			t.Fatalf("register %q: %v", name, err)
		}
	}
	recentItems := make([]config.RecentPaletteItem, 9)
	for i := range recentItems {
		recentItems[i] = config.RecentPaletteItem{Category: string(ItemCategoryCommands), ID: fmt.Sprintf("session15 command %02d", i)}
	}
	app.runtime.Machine().SetRecentPaletteItems(recentItems)

	palette := NewCommandPaletteModel(app.runtime, registry, ViewDashboard, nil)
	if got := len(palette.filteredItems); got != 12 {
		t.Fatalf("empty palette items = %d, want 12", got)
	}

	recent, contextual := 0, 0
	seen := make(map[string]bool)
	for _, filtered := range palette.filteredItems {
		if filtered.Item.Category != ItemCategoryCommands || filtered.Item.Command == nil {
			t.Fatalf("empty palette exposed non-command item: %+v", filtered.Item)
		}
		if filtered.IsFrequent {
			recent++
		}
		if filtered.IsSuggested {
			contextual++
		}
		name := filtered.Item.Command.Name
		if seen[name] {
			t.Fatalf("empty palette duplicated command %q", name)
		}
		seen[name] = true
	}
	if recent != maxEmptyPaletteRecentCommands || contextual != maxEmptyPaletteContextualCommands {
		t.Fatalf("empty palette sections = recent %d contextual %d, want %d/%d",
			recent, contextual, maxEmptyPaletteRecentCommands, maxEmptyPaletteContextualCommands)
	}
}

func TestSession15PaletteTypedSearchCoversSupportedEntities(t *testing.T) {
	app := newTestAppModel()
	palette := NewCommandPaletteModel(app.runtime, NewCommandRegistry(), ViewDashboard, nil)
	project := &models.Project{ID: "project", Name: "Needle Project", Slug: "needle-project"}
	task := &models.Task{ID: "task", Title: "Needle Task", ShortID: 42}
	note := models.NewNote("Needle Note")
	palette.allItems = []PaletteItem{
		{Name: "Needle Command", Category: ItemCategoryCommands, Command: &Command{Name: "Needle Command"}},
		{Name: project.Name, Category: ItemCategoryProjects, Project: project},
		{Name: task.Title, Category: ItemCategoryTasks, Task: task, Project: project},
		{Name: note.Title, Category: ItemCategoryNotes, Note: note, Project: project},
		{Name: "Needle Filter", Category: ItemCategorySavedFilter},
		{Name: "Needle Tag", Category: ItemCategoryTags, Tag: "needle"},
		{Name: "Needle Section", Category: ItemCategorySections, Section: "needle"},
		{Name: "Needle View", Category: ItemCategoryViews, ViewTarget: ViewDashboard},
	}
	palette.searchInput.SetValue("needle")
	palette.filterItems()

	want := map[ItemCategory]bool{
		ItemCategoryCommands: true, ItemCategoryProjects: true, ItemCategoryTasks: true,
		ItemCategoryNotes: true, ItemCategorySavedFilter: true, ItemCategoryTags: true,
		ItemCategorySections: true, ItemCategoryViews: true,
	}
	for _, filtered := range palette.filteredItems {
		delete(want, filtered.Item.Category)
	}
	if len(want) != 0 {
		t.Fatalf("typed palette search missed categories: %v", want)
	}
}

func TestSession15PaletteRetiresLegacyControlsAndGhostCommands(t *testing.T) {
	app := newTestAppModel()
	registry := NewCommandRegistry()
	if err := registry.RegisterBatch(app.globalCommands()); err != nil {
		t.Fatalf("register global commands: %v", err)
	}
	palette := NewCommandPaletteModel(app.runtime, registry, ViewDashboard, nil)

	for _, item := range palette.allItems {
		if item.Command == nil {
			continue
		}
		if item.Command.Name == "global search" && item.Shortcut != "" {
			t.Fatalf("global search restored retired shortcut %q", item.Shortcut)
		}
		if item.Command.Name == "help" && item.Shortcut != "?" {
			t.Fatalf("Help shortcut = %q, want ?", item.Shortcut)
		}
	}

	palette.searchInput.SetValue("+create from the old inline grammar")
	palette.filterItems()
	if len(palette.filteredItems) != 0 {
		t.Fatal("retired inline task grammar produced palette results")
	}

	palette.searchInput.SetValue("")
	palette.filterItems()
	if _, cmd := palette.Update(tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl}); cmd != nil {
		t.Fatal("ctrl+\\ still opens a palette reference surface")
	}
}

func TestSession15PaletteAliasInventoryAndTaskPeek(t *testing.T) {
	app := newTestAppModel()
	palette := NewCommandPaletteModel(app.runtime, NewCommandRegistry(), ViewDashboard, nil)
	wantAliases := map[PresentationID]string{
		ViewGlobalSearch: "Global Search",
		ViewNotesSearch:  "Notes Search",
	}
	for _, item := range palette.allItems {
		if want, ok := wantAliases[item.ViewTarget]; ok && item.Name == want {
			delete(wantAliases, item.ViewTarget)
		}
	}
	if len(wantAliases) != 0 {
		t.Fatalf("palette is missing legacy view aliases: %v", wantAliases)
	}

	project := &models.Project{ID: "project", Name: "Project"}
	task := &models.Task{ID: "task", Title: "Task"}
	palette.filteredItems = []filteredPaletteItem{{Item: PaletteItem{
		Name: task.Title, Category: ItemCategoryTasks, Task: task, Project: project,
	}}}
	_, cmd := palette.updateSearchMode(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if cmd == nil {
		t.Fatal("Shift+Enter did not produce a Peek command")
	}
	peek, ok := cmd().(PalettePeekRequestMsg)
	if !ok {
		t.Fatalf("Shift+Enter message = %T, want PalettePeekRequestMsg", cmd())
	}
	if peek.Task != task || peek.Project != project {
		t.Fatalf("Shift+Enter result = %#v, want in-app Peek of task/project", peek)
	}
}

func TestSession15RecentAndPinnedPeekNestAboveTheirOverlay(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind OverlayKind
		key  string
		open func(*AppModel, *models.Task, *models.Project)
		peek func(*AppModel)
	}{
		{
			name: "recent", kind: OverlayRecent, key: "recent-tasks",
			open: func(app *AppModel, task *models.Task, project *models.Project) {
				app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: project}}
				app.quickSwitchFiltered = []int{0}
				app.showQuickSwitch = true
			},
			peek: func(app *AppModel) {
				app.handleQuickSwitchNormalKey(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
			},
		},
		{
			name: "pinned", kind: OverlayPinned, key: "pinned-tasks",
			open: func(app *AppModel, task *models.Task, project *models.Project) {
				app.pinnedPopupItems = []pinnedTaskInfo{{Task: task, Project: project, Title: task.Title, ProjectName: project.Name}}
				app.pinnedPopupFiltered = []int{0}
				app.showPinnedPopup = true
			},
			peek: func(app *AppModel) {
				app.handlePinnedPopupNormalKey(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppModel()
			app.enableTypedNavigation()
			project := &models.Project{ID: "project", Name: "Project"}
			task := &models.Task{ID: "task", Title: "Task"}
			tc.open(app, task, project)
			app.pushTypedOverlay(tc.kind, tc.key)
			tc.peek(app)

			if !app.peekActive || app.navigation.Overlays.Depth() != 2 {
				t.Fatalf("nested Peek = active %v depth %d, want true/2", app.peekActive, app.navigation.Overlays.Depth())
			}
			if top, _ := app.navigation.Overlays.Current(); top.Kind() != OverlayPeek {
				t.Fatalf("top overlay = %s, want Peek", top.Kind())
			}
			app.dismissPeek()
			if top, _ := app.navigation.Overlays.Current(); top.Kind() != tc.kind {
				t.Fatalf("overlay after Peek close = %s, want %s", top.Kind(), tc.kind)
			}
		})
	}
}

func TestSession15PinnedUsesPForUnpinAndRecentMutationKeysAreInert(t *testing.T) {
	pinned, _, _, _ := pinnedPopupFixture(t)
	before := len(pinned.pinnedPopupItems)
	pinned.handlePinnedPopupNormalKey(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if got := len(pinned.pinnedPopupItems); got != before-1 {
		t.Fatalf("p unpin count = %d, want %d", got, before-1)
	}
	afterUnpin := len(pinned.pinnedPopupItems)
	pinned.handlePinnedPopupNormalKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if len(pinned.pinnedPopupItems) != afterUnpin {
		t.Fatal("retired repin key restored a task")
	}

	recent := newTestAppForQuickSwitch()
	task := &models.Task{ID: "task", Title: "Task"}
	project := &models.Project{ID: "project", Name: "Project"}
	recent.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: project}}
	recent.quickSwitchFiltered = []int{0}
	recent.showQuickSwitch = true
	for _, key := range []rune{',', '!', 'c', 'x', '1'} {
		recent.handleQuickSwitchNormalKey(tea.KeyPressMsg{Code: key, Text: string(key)})
	}
	if !recent.showQuickSwitch || len(recent.recentTaskVisits) != 1 {
		t.Fatalf("retired Recent controls changed state: open=%v visits=%d",
			recent.showQuickSwitch, len(recent.recentTaskVisits))
	}
}

func TestSession15TypedReferenceAskCancellationAndOverlayManager(t *testing.T) {
	app := newTestAppModel()
	app.enableTypedNavigation()
	ask := NewAskViewModelWithDeps(nil, "", nil, nil)
	canceled := false
	ask.state = askStateLoading
	ask.cancelFn = func() { canceled = true }
	app.views.Register(ViewAsk, ask)
	app.pushView(ViewAsk)

	top, ok := app.navigation.Overlays.Current()
	if !ok || top.Kind() != OverlayReference {
		t.Fatalf("Ask overlay = %v/%v, want typed Reference", top.Kind(), ok)
	}
	if route, ok := top.ReferenceRoute(); !ok || route.Params().(ReferenceParams).Kind != ReferenceAsk {
		t.Fatalf("Ask reference route = %#v/%v", route, ok)
	}
	app.popView()
	if !canceled || ask.cancelFn != nil || ask.state != askStateInput {
		t.Fatalf("typed Ask close cleanup = canceled %v cancelFn %v state %v", canceled, ask.cancelFn != nil, ask.state)
	}

	app.showQuickSwitch = true
	app.pushTypedOverlay(OverlayRecent, "recent-tasks")
	if !app.Overlays().Active() {
		t.Fatal("OverlayManager did not observe typed Recent")
	}
	app.Overlays().ClearAll()
	if app.Overlays().Active() || app.navigation.Overlays.Depth() != 0 || app.showQuickSwitch {
		t.Fatalf("OverlayManager clear = active %v depth %d recent %v",
			app.Overlays().Active(), app.navigation.Overlays.Depth(), app.showQuickSwitch)
	}
}

func TestSession15GlobalHelpHasNoGhostBindings(t *testing.T) {
	items := getGlobalHelpItems()
	if len(items) != 9 {
		t.Fatalf("global Help bindings = %d, want 9", len(items))
	}
	keys := make(map[string]bool, len(items))
	for _, item := range items {
		keys[strings.ToLower(item.Key)] = true
	}
	for _, ghost := range []string{`ctrl+\`, "ctrl+/", "alt+o", "alt+i", "ctrl+6", "alt+l", ";"} {
		if keys[ghost] {
			t.Errorf("global Help contains ghost binding %q", ghost)
		}
	}
}
