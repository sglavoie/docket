package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
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
	project := ws.Runtime.TaskWorkspaceProjects()[0]
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
	if got := snapString(t, snap, "mode"); got != string(chrome.ModeAll) {
		t.Errorf("mode = %q, want %s", got, chrome.ModeAll)
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
	driver := NewUITestDriver(app, 100, 30)

	cmds := driver.availableCommands()
	var found *UITestCommand
	for index := range cmds {
		if cmds[index].ID == string(operation.OperationCoreCapture) {
			found = &cmds[index]
			break
		}
	}
	if found == nil {
		t.Fatalf("capture operation is missing from available commands: %+v", cmds)
	}
	if found.Name != "Capture" || found.Shortcut != "a" || found.Category != "Tasks" {
		t.Errorf("capture operation fields = %+v", *found)
	}
	if len(found.Aliases) == 0 {
		t.Errorf("capture operation aliases = %v, want at least one", found.Aliases)
	}
}

func TestUITestAvailableCommandsUsesSimplifiedOperationStrategy(t *testing.T) {
	app := newUITestApp()

	commands := NewUITestDriver(app, 100, 30).availableCommands()
	strategy, _ := app.operationRegistry.InputStrategy(operation.InputStrategySimplified)
	bound := make(map[operation.OperationID]struct{}, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		bound[binding.OperationID] = struct{}{}
	}
	byID := make(map[string]UITestCommand, len(commands))
	for _, command := range commands {
		byID[command.ID] = command
		// A preset family declares one binding per value, so the driver reports one
		// command per operation instead of one per binding.
		if _, ok := bound[operation.OperationID(command.ID)]; !ok {
			t.Errorf("command %s is not bound by the simplified strategy", command.ID)
		}
	}
	if len(commands) != len(byID) {
		t.Errorf("available operations = %d, distinct IDs = %d", len(commands), len(byID))
	}
	if got := byID[string(operation.OperationCoreScopeSearch)].Shortcut; got != "/" {
		t.Errorf("scope search shortcut = %q, want /", got)
	}
}
