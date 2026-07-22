package tui

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// newScoringPalette builds a bare palette with a real search input and the
// given item set, bypassing the runtime-backed loader so scoring, show-all, and
// navigation can be exercised in isolation. It clears the process-global recent
// selections so score assertions are deterministic.
func newScoringPalette(items []PaletteItem) *CommandPaletteModel {
	recentPaletteSelections = nil
	si := textinput.New()
	m := &CommandPaletteModel{
		searchInput:   si,
		scrollAnchor:  -1,
		selectedTasks: make(map[string]bool),
	}
	m.allItems = items
	m.filterItems()
	return m
}

// TestSession5PalettePeekActivatesInAppOverlayAndEscRestores pins that a palette
// Peek request opens the AppModel's in-app Peek overlay while leaving the
// command palette on the view stack, and that Esc dismisses only the Peek —
// restoring the palette beneath it.
func TestSession5PalettePeekActivatesInAppOverlayAndEscRestores(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []PresentationID{ViewDashboard, ViewCommandPalette}

	project := &models.Project{ID: "p", Name: "Proj", Slug: "proj"}
	task := &models.Task{ID: "t", Title: "Task"}

	handled, cmd := app.handleSearchMessages(PalettePeekRequestMsg{Task: task, Project: project})
	if !handled {
		t.Fatal("PalettePeekRequestMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("peek activation should not emit a cmd, got %T", cmd())
	}
	if !app.peekActive || app.peekTask != task || app.peekProject != project {
		t.Fatalf("peek not activated: active=%v task=%v project=%v", app.peekActive, app.peekTask, app.peekProject)
	}
	if app.currentView() != ViewCommandPalette {
		t.Fatalf("palette must remain on stack under Peek, currentView=%v", app.currentView())
	}

	escHandled, _ := app.handleEscKey()
	if !escHandled {
		t.Fatal("Esc should be handled while Peek is active")
	}
	if app.peekActive {
		t.Fatal("Esc should dismiss the Peek overlay")
	}
	if app.currentView() != ViewCommandPalette {
		t.Fatalf("palette must be restored after Peek closes, currentView=%v", app.currentView())
	}
}

// TestSession5PeekItemEmitsInAppRequest pins that Shift+Enter on a task produces
// a dedicated in-app Peek request (not a Glow view) carrying the task/project.
func TestSession5PeekItemEmitsInAppRequest(t *testing.T) {
	project := &models.Project{ID: "p", Name: "Proj", Slug: "proj"}
	task := &models.Task{ID: "t", Title: "Task"}
	m := newScoringPalette(nil)
	m.filteredItems = []filteredPaletteItem{{Item: PaletteItem{
		Name: task.Title, Category: ItemCategoryTasks, Task: task, Project: project,
	}}}

	_, cmd := m.updateSearchMode(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if cmd == nil {
		t.Fatal("Shift+Enter should produce a Peek command")
	}
	peek, ok := cmd().(PalettePeekRequestMsg)
	if !ok {
		t.Fatalf("Shift+Enter = %T, want PalettePeekRequestMsg", cmd())
	}
	if peek.Task != task || peek.Project != project {
		t.Fatalf("peek request = %#v, want task/project", peek)
	}
}

// TestSession5ShowAllToggleAndPersistence pins the transient show-all behavior:
// Ctrl+A on an empty query expands to every item, toggles back, persists across
// query typing/clearing, and stays inert (ordinary text input) for a nonempty
// query.
func TestSession5ShowAllToggleAndPersistence(t *testing.T) {
	items := []PaletteItem{
		{Name: "Alpha Command", Category: ItemCategoryCommands, Command: &Command{Name: "Alpha Command"}},
		{Name: "Beta Command", Category: ItemCategoryCommands, Command: &Command{Name: "Beta Command"}},
		{Name: "Gamma Project", Category: ItemCategoryProjects, Project: &models.Project{Name: "Gamma Project", Slug: "gamma"}},
	}

	m := newScoringPalette(items)
	// Bounded empty palette lists commands only, never the project entity.
	for _, fi := range m.filteredItems {
		if fi.Item.Category == ItemCategoryProjects {
			t.Fatal("bounded empty palette should not include project entities")
		}
	}

	// Ctrl+A on empty query expands to include the project.
	m.updateSearchMode(ctrlKey('a'))
	if !m.showAll {
		t.Fatal("Ctrl+A on empty query should enable show-all")
	}
	if !containsCategory(m.filteredItems, ItemCategoryProjects) {
		t.Fatal("show-all should surface the project entity")
	}

	// Persists across typing a query and clearing it back to empty.
	m.searchInput.SetValue("zzz-no-match")
	m.filterItems()
	m.searchInput.SetValue("")
	m.filterItems()
	if !m.showAll || !containsCategory(m.filteredItems, ItemCategoryProjects) {
		t.Fatal("show-all should persist across query clearing")
	}

	// Ctrl+A again collapses back to the bounded set.
	m.updateSearchMode(ctrlKey('a'))
	if m.showAll {
		t.Fatal("second Ctrl+A should disable show-all")
	}
	if containsCategory(m.filteredItems, ItemCategoryProjects) {
		t.Fatal("collapsed palette should drop the project entity again")
	}

	// Ctrl+A with a nonempty query is ordinary text input (no toggle).
	m.searchInput.SetValue("ab")
	m.filterItems()
	m.updateSearchMode(ctrlKey('a'))
	if m.showAll {
		t.Fatal("Ctrl+A with a nonempty query must not toggle show-all")
	}
	if m.searchInput.Value() != "ab" {
		t.Fatalf("nonempty Ctrl+A should not alter the query, got %q", m.searchInput.Value())
	}
}

// TestSession5CtrlJKNavigationBoundaries pins non-wrapping Ctrl+J/Ctrl+K
// navigation in both search and action modes.
func TestSession5CtrlJKNavigationBoundaries(t *testing.T) {
	m := newScoringPalette(nil)
	m.filteredItems = []filteredPaletteItem{
		{Item: PaletteItem{Name: "a", Category: ItemCategoryCommands, Command: &Command{Name: "a"}}},
		{Item: PaletteItem{Name: "b", Category: ItemCategoryCommands, Command: &Command{Name: "b"}}},
	}

	// Ctrl+K at the top does not wrap.
	m.cursor = 0
	m.updateSearchMode(ctrlKey('k'))
	if m.cursor != 0 {
		t.Fatalf("Ctrl+K at top = %d, want 0 (no wrap)", m.cursor)
	}
	// Ctrl+J advances.
	m.updateSearchMode(ctrlKey('j'))
	if m.cursor != 1 {
		t.Fatalf("Ctrl+J = %d, want 1", m.cursor)
	}
	// Ctrl+J at the bottom does not wrap.
	m.updateSearchMode(ctrlKey('j'))
	if m.cursor != 1 {
		t.Fatalf("Ctrl+J at bottom = %d, want 1 (no wrap)", m.cursor)
	}

	// Action mode: same boundaries on actionCursor.
	m.actionMode = true
	m.availableActions = []PaletteAction{{Name: "one"}, {Name: "two"}}
	m.actionCursor = 0
	m.updateActionMode(ctrlKey('k'))
	if m.actionCursor != 0 {
		t.Fatalf("action Ctrl+K at top = %d, want 0", m.actionCursor)
	}
	m.updateActionMode(ctrlKey('j'))
	if m.actionCursor != 1 {
		t.Fatalf("action Ctrl+J = %d, want 1", m.actionCursor)
	}
	m.updateActionMode(ctrlKey('j'))
	if m.actionCursor != 1 {
		t.Fatalf("action Ctrl+J at bottom = %d, want 1 (no wrap)", m.actionCursor)
	}
}

// TestSession5ExactProjectIdentityOutranksFuzzyCommands pins that an exactly
// named or slugged project cannot be buried beneath command fuzzy matches — the
// motivating "tt" case in a command-heavy context.
func TestSession5ExactProjectIdentityOutranksFuzzyCommands(t *testing.T) {
	items := []PaletteItem{
		{Name: "Toggle Tree", Category: ItemCategoryCommands, Command: &Command{Name: "Toggle Tree"}},
		{Name: "Test Task", Category: ItemCategoryCommands, Command: &Command{Name: "Test Task"}},
		{Name: "Trim Tags", Category: ItemCategoryCommands, Command: &Command{Name: "Trim Tags"}},
		{Name: "tt", Category: ItemCategoryProjects, Project: &models.Project{Name: "tt", Slug: "tt"}},
	}
	m := newScoringPalette(items)
	m.searchInput.SetValue("tt")
	m.filterItems()

	if len(m.filteredItems) == 0 {
		t.Fatal("query 'tt' produced no results")
	}
	top := m.filteredItems[0].Item
	if top.Category != ItemCategoryProjects || top.Project == nil || top.Project.Slug != "tt" {
		t.Fatalf("top result = %q (%s), want project 'tt'", top.Name, top.Category)
	}

	// Slug-only exact match also wins even when the name does not match.
	items2 := []PaletteItem{
		{Name: "Toggle Tree", Category: ItemCategoryCommands, Command: &Command{Name: "Toggle Tree"}},
		{Name: "Time Tracker", Category: ItemCategoryProjects, Project: &models.Project{Name: "Time Tracker", Slug: "tt"}},
	}
	m2 := newScoringPalette(items2)
	m2.searchInput.SetValue("tt")
	m2.filterItems()
	if len(m2.filteredItems) == 0 || m2.filteredItems[0].Item.Category != ItemCategoryProjects {
		t.Fatal("exact slug 'tt' should top-rank the project even with a non-matching name")
	}
}

// TestSession5ProjectMatchQualityOrdering pins the exact > prefix > substring >
// fuzzy ranking of project identity matches and the slug match provenance.
func TestSession5ProjectMatchQualityOrdering(t *testing.T) {
	if got := projectMatchQuality("Alpha", "ser", "ser"); got != projectQualityExact {
		t.Fatalf("exact slug quality = %d, want %d", got, projectQualityExact)
	}
	if got := projectMatchQuality("Alpha", "server", "ser"); got != projectQualityPrefix {
		t.Fatalf("prefix slug quality = %d, want %d", got, projectQualityPrefix)
	}
	if got := projectMatchQuality("Alpha", "webserver", "ser"); got != projectQualitySubstring {
		t.Fatalf("substring slug quality = %d, want %d", got, projectQualitySubstring)
	}
	if got := projectMatchQuality("Alpha", "seer", "ser"); got != projectQualityFuzzy {
		t.Fatalf("fuzzy slug quality = %d, want %d", got, projectQualityFuzzy)
	}
	if got := projectMatchQuality("Alpha", "beta", "ser"); got != projectQualityNone {
		t.Fatalf("no-match quality = %d, want %d", got, projectQualityNone)
	}

	items := []PaletteItem{
		{Name: "Fuzzy One", Category: ItemCategoryProjects, Project: &models.Project{Name: "Fuzzy One", Slug: "seer"}},
		{Name: "Substr One", Category: ItemCategoryProjects, Project: &models.Project{Name: "Substr One", Slug: "webserver"}},
		{Name: "Prefix One", Category: ItemCategoryProjects, Project: &models.Project{Name: "Prefix One", Slug: "server"}},
		{Name: "Exact One", Category: ItemCategoryProjects, Project: &models.Project{Name: "Exact One", Slug: "ser"}},
	}
	m := newScoringPalette(items)
	m.searchInput.SetValue("ser")
	m.filterItems()

	wantSlugs := []string{"ser", "server", "webserver", "seer"}
	if len(m.filteredItems) != len(wantSlugs) {
		t.Fatalf("got %d results, want %d", len(m.filteredItems), len(wantSlugs))
	}
	for i, want := range wantSlugs {
		got := m.filteredItems[i].Item.Project.Slug
		if got != want {
			t.Fatalf("result[%d] slug = %q, want %q (full order %v)", i, got, want, slugOrder(m.filteredItems))
		}
	}
}

// TestSession5SlugMatchProvenance pins that a slug-only match records
// MatchFieldProjectSlug so the preview can attribute the result to the slug.
func TestSession5SlugMatchProvenance(t *testing.T) {
	items := []PaletteItem{
		{Name: "Backend Service", Category: ItemCategoryProjects, Project: &models.Project{Name: "Backend Service", Slug: "api"}},
	}
	m := newScoringPalette(items)
	m.searchInput.SetValue("api")
	m.filterItems()

	if len(m.filteredItems) != 1 {
		t.Fatalf("got %d results, want 1", len(m.filteredItems))
	}
	if got := m.filteredItems[0].MatchedField; got != MatchFieldProjectSlug {
		t.Fatalf("matched field = %v, want MatchFieldProjectSlug", got)
	}
	// renderMatchInfo surfaces the slug provenance.
	info := m.renderMatchInfo(m.filteredItems[0])
	if info == "" {
		t.Fatal("slug match should render match-info provenance")
	}
}

func containsCategory(items []filteredPaletteItem, cat ItemCategory) bool {
	for _, fi := range items {
		if fi.Item.Category == cat {
			return true
		}
	}
	return false
}

func slugOrder(items []filteredPaletteItem) []string {
	out := make([]string, 0, len(items))
	for _, fi := range items {
		if fi.Item.Project != nil {
			out = append(out, fi.Item.Project.Slug)
		}
	}
	return out
}
