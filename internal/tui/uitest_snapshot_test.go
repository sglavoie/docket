package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/storage"
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
	if err := ws.persistSyncedDomains(); err != nil {
		t.Fatalf("save domains: %v", err)
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

func TestUITestSnapshotOutliner(t *testing.T) {
	ws := newSnapshotWorkspace(t)
	project := ws.Runtime.ActiveProjects()[0]
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load project: %v", err)
	}
	m := NewOutlinerModel(project, ws.Manager, ws.Runtime)
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

func TestUITestSnapshotCommandPalette(t *testing.T) {
	ws := newSnapshotWorkspace(t)
	registry := NewCommandRegistry()
	_ = registry.Register(Command{Name: "Create Task", Shortcut: "n", Category: CategoryTasks})
	m := NewCommandPaletteModel(ws.Runtime, registry, ViewDashboard, nil)

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
	// The Outliner is the sole root, so the fixture opens on the global
	// (cross-project) Outliner and exposes its own semantic snapshot.
	if obs.CurrentView != "CrossProjectOutliner" {
		t.Fatalf("CurrentView = %q, want CrossProjectOutliner", obs.CurrentView)
	}
	if obs.Snapshot == nil {
		t.Fatal("Snapshot is nil for the Outliner root view")
	}
	if crossProject, ok := obs.Snapshot["cross_project"]; !ok || crossProject != true {
		t.Errorf("snapshot missing cross_project=true; got %v", obs.Snapshot)
	}
	if _, ok := obs.Snapshot["row_count"]; !ok {
		t.Errorf("snapshot missing row_count; got %v", obs.Snapshot)
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
		Views:    []PresentationID{ViewCrossProjectOutliner},
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

func TestUITestAvailableCommandsOmitsUnavailableCommands(t *testing.T) {
	app := newUITestApp()
	app.commandRegistry = NewCommandRegistry()
	dynamicAvailable := true
	for _, cmd := range []Command{
		{Name: "Always", Category: CategoryTasks},
		{Name: "Dynamic", Category: CategoryTasks, Available: func() bool { return dynamicAvailable }},
		{Name: "Never", Category: CategoryTasks, Available: func() bool { return false }},
	} {
		if err := app.commandRegistry.Register(cmd); err != nil {
			t.Fatalf("Register(%q): %v", cmd.Name, err)
		}
	}
	driver := NewUITestDriver(app, 100, 30)

	commandNames := func(commands []UITestCommand) []string {
		names := make([]string, 0, len(commands))
		for _, cmd := range commands {
			names = append(names, cmd.Name)
		}
		return names
	}
	if got := commandNames(driver.availableCommands()); len(got) != 2 || got[0] != "Always" || got[1] != "Dynamic" {
		t.Fatalf("available commands = %v, want [Always Dynamic]", got)
	}

	dynamicAvailable = false
	if got := commandNames(driver.availableCommands()); len(got) != 1 || got[0] != "Always" {
		t.Fatalf("available commands after predicate change = %v, want [Always]", got)
	}
}

func TestCommandPaletteOmitsUnavailableCommands(t *testing.T) {
	runtime := newTestConfig()
	registry := NewCommandRegistry()
	for _, cmd := range []Command{
		{Name: "Always", Category: CategoryTasks},
		{Name: "Never", Category: CategoryTasks, Available: func() bool { return false }},
	} {
		if err := registry.Register(cmd); err != nil {
			t.Fatalf("Register(%q): %v", cmd.Name, err)
		}
	}

	palette := NewCommandPaletteModel(runtime, registry, ViewDashboard, nil)
	found := make(map[string]bool)
	for _, item := range palette.allItems {
		if item.Command != nil {
			found[item.Command.Name] = true
		}
	}
	if !found["Always"] {
		t.Fatal("palette omitted an available command")
	}
	if found["Never"] {
		t.Fatal("palette included an unavailable command")
	}
}
