package tui

import (
	"testing"
)

// newSnapshotWorkspace builds an isolated workspace preloaded with the rich
// fixture and persists it, so the view models under test load real task data.
func newSnapshotWorkspace(t *testing.T) *uitestWorkspace {
	t.Helper()
	ws, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("newUITestWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })
	if err := buildRichFixture(ws); err != nil {
		t.Fatalf("buildRichFixture: %v", err)
	}
	if err := ws.Manager.Save(ws.Config); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return ws
}

func snapString(t *testing.T, snap map[string]any, key string) string {
	t.Helper()
	v, ok := snap[key]
	if !ok {
		t.Fatalf("snapshot missing key %q; got %v", key, snap)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("snapshot key %q = %T, want string", key, v)
	}
	return s
}

func snapInt(t *testing.T, snap map[string]any, key string) int {
	t.Helper()
	v, ok := snap[key]
	if !ok {
		t.Fatalf("snapshot missing key %q; got %v", key, snap)
	}
	n, ok := v.(int)
	if !ok {
		t.Fatalf("snapshot key %q = %T, want int", key, v)
	}
	return n
}

func TestUITestSnapshotDashboard(t *testing.T) {
	ws := newSnapshotWorkspace(t)
	m := NewDashboardModel(ws.Config, ws.Manager)

	snap := m.UITestSnapshot()
	if got := snapInt(t, snap, "project_count"); got != 1 {
		t.Errorf("project_count = %d, want 1", got)
	}
	if got := snapInt(t, snap, "cursor"); got != 0 {
		t.Errorf("cursor = %d, want 0", got)
	}
	if got := snapString(t, snap, "selected_project"); got != "Rich Project" {
		t.Errorf("selected_project = %q, want Rich Project", got)
	}
	if _, ok := snap["show_archived"]; !ok {
		t.Error("snapshot missing show_archived")
	}
}

func TestUITestSnapshotFocusBoard(t *testing.T) {
	ws := newSnapshotWorkspace(t)
	dailyPlan := []FocusTask{}
	m := NewFocusBoardModel(ws.Config, ws.Manager, &dailyPlan,
		map[string]bool{}, map[string]bool{}, map[string]bool{})

	snap := m.UITestSnapshot()
	if _, ok := snap["row_count"]; !ok {
		t.Error("snapshot missing row_count")
	}
	if _, ok := snap["cursor"]; !ok {
		t.Error("snapshot missing cursor")
	}
	if got := snapString(t, snap, "active_tab"); got == "" {
		t.Error("active_tab is empty")
	}
}

func TestUITestSnapshotOutliner(t *testing.T) {
	ws := newSnapshotWorkspace(t)
	project := ws.Config.Projects[0]
	m := NewOutlinerModel(project, ws.Config, ws.Manager)
	// Rows are rebuilt on render; mirror the driver flow (Observe renders the
	// active view before reading its snapshot) so the snapshot has rows.
	m.SetSize(100, 30)
	_ = m.View()
	// Park the cursor on the first task row (row 0 is a section header) so the
	// selected_task field is populated.
	for i := range m.rows {
		if m.rows[i].task != nil {
			m.cursor = i
			break
		}
	}

	snap := m.UITestSnapshot()
	if got := snapString(t, snap, "perspective"); got != "All" {
		t.Errorf("perspective = %q, want All", got)
	}
	if got := snapInt(t, snap, "row_count"); got <= 0 {
		t.Errorf("row_count = %d, want > 0", got)
	}
	if got, ok := snap["cross_project"].(bool); !ok || got {
		t.Errorf("cross_project = %v, want false", snap["cross_project"])
	}
	// The rich fixture has tasks, so the cursor row should carry a title.
	if got := snapString(t, snap, "selected_task"); got == "" {
		t.Error("selected_task is empty")
	}
}

func TestUITestSnapshotKanban(t *testing.T) {
	ws := newSnapshotWorkspace(t)
	project := ws.Config.Projects[0]
	m := NewKanbanModel(project, ws.Config, ws.Manager, NewProjectService())

	snap := m.UITestSnapshot()
	if got := snapString(t, snap, "mode"); got != "Section" {
		t.Errorf("mode = %q, want Section", got)
	}
	if got := snapInt(t, snap, "column_count"); got <= 0 {
		t.Errorf("column_count = %d, want > 0", got)
	}
	if got := snapInt(t, snap, "card_count"); got <= 0 {
		t.Errorf("card_count = %d, want > 0", got)
	}
	if got, ok := snap["cross_project"].(bool); !ok || got {
		t.Errorf("cross_project = %v, want false", snap["cross_project"])
	}
}

func TestUITestSnapshotCommandPalette(t *testing.T) {
	ws := newSnapshotWorkspace(t)
	registry := NewCommandRegistry()
	_ = registry.Register(Command{Name: "Create Task", Shortcut: "n", Category: CategoryTasks})
	m := NewCommandPaletteModel(ws.Config, ws.Manager, registry, ViewDashboard, nil)

	snap := m.UITestSnapshot()
	if _, ok := snap["query"]; !ok {
		t.Error("snapshot missing query")
	}
	if _, ok := snap["result_count"]; !ok {
		t.Error("snapshot missing result_count")
	}
	if got, ok := snap["action_mode"].(bool); !ok || got {
		t.Errorf("action_mode = %v, want false", snap["action_mode"])
	}
}

// TestUITestSnapshotObservation proves the driver wires the active view's
// snapshot into the observation end to end.
func TestUITestSnapshotObservation(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureRich, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	defer func() { _ = ws.Cleanup() }()

	obs := driver.Observe(ObserveOptions{})
	if obs.CurrentView != "Dashboard" {
		t.Fatalf("CurrentView = %q, want Dashboard", obs.CurrentView)
	}
	if obs.Snapshot == nil {
		t.Fatal("Snapshot is nil for the dashboard view")
	}
	if _, ok := obs.Snapshot["project_count"]; !ok {
		t.Errorf("snapshot missing project_count; got %v", obs.Snapshot)
	}
}

func TestUITestAvailableCommands(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureRich, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	defer func() { _ = ws.Cleanup() }()

	obs := driver.Observe(ObserveOptions{})
	if len(obs.AvailableCommands) == 0 {
		t.Fatal("AvailableCommands is empty for the dashboard view")
	}

	// Every command carries a name and the list is sorted by name for stability.
	prev := ""
	for i, c := range obs.AvailableCommands {
		if c.Name == "" {
			t.Errorf("command %d has empty name", i)
		}
		if prev != "" && c.Name < prev {
			t.Errorf("commands not sorted by name: %q before %q", prev, c.Name)
		}
		prev = c.Name
	}
}

// TestUITestAvailableCommandsFields verifies name/shortcut/category/aliases are
// carried through from the registry entry.
func TestUITestAvailableCommandsFields(t *testing.T) {
	app := newUITestApp()
	app.commandRegistry = NewCommandRegistry()
	if err := app.commandRegistry.Register(Command{
		Name:     "Create Task",
		Shortcut: "n",
		Category: CategoryTasks,
		Aliases:  []string{"add task", "new task"},
		Views:    []ViewType{ViewDashboard},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	driver := NewUITestDriver(app, 100, 30)

	cmds := driver.availableCommands()
	if len(cmds) != 1 {
		t.Fatalf("availableCommands len = %d, want 1", len(cmds))
	}
	c := cmds[0]
	if c.Name != "Create Task" || c.Shortcut != "n" || c.Category != "Tasks" {
		t.Errorf("command fields = %+v", c)
	}
	if len(c.Aliases) != 2 || c.Aliases[0] != "add task" {
		t.Errorf("aliases = %v", c.Aliases)
	}
}
