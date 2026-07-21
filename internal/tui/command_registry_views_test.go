package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRegisterSameNameDifferentViewsSucceeds(t *testing.T) {
	registry := NewCommandRegistry()

	cmd1 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewTaskDetail},
		Handler:  func() tea.Msg { return nil },
	}
	cmd2 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewOutliner},
		Handler:  func() tea.Msg { return nil },
	}

	if err := registry.Register(cmd1); err != nil {
		t.Fatalf("expected first Register to succeed, got: %v", err)
	}
	if err := registry.Register(cmd2); err != nil {
		t.Fatalf("expected second Register with different views to succeed, got: %v", err)
	}
}

func TestRegisterSameNameOverlappingViewsFails(t *testing.T) {
	registry := NewCommandRegistry()

	cmd1 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewTaskDetail, ViewOutliner},
		Handler:  func() tea.Msg { return nil },
	}
	cmd2 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewOutliner, ViewDashboard},
		Handler:  func() tea.Msg { return nil },
	}

	if err := registry.Register(cmd1); err != nil {
		t.Fatalf("expected first Register to succeed, got: %v", err)
	}

	err := registry.Register(cmd2)
	if err == nil {
		t.Error("expected error when registering command with overlapping views")
	}
}

func TestRegisterSameNameGlobalConflicts(t *testing.T) {
	registry := NewCommandRegistry()

	globalCmd := Command{
		Name:     "Help",
		Category: CategoryHelp,
		Handler:  func() tea.Msg { return nil },
	}
	viewCmd := Command{
		Name:     "Help",
		Category: CategoryHelp,
		Views:    []PresentationID{ViewTaskDetail},
		Handler:  func() tea.Msg { return nil },
	}

	if err := registry.Register(globalCmd); err != nil {
		t.Fatalf("expected global Register to succeed, got: %v", err)
	}

	err := registry.Register(viewCmd)
	if err == nil {
		t.Error("expected error when registering view-specific command that conflicts with global")
	}
}

func TestGetForViewReturnsCorrectVariant(t *testing.T) {
	registry := NewCommandRegistry()

	type resultMsg struct{ view string }

	cmd1 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewTaskDetail},
		Handler:  func() tea.Msg { return resultMsg{"tree"} },
	}
	cmd2 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewOutliner},
		Handler:  func() tea.Msg { return resultMsg{"outliner"} },
	}

	_ = registry.Register(cmd1)
	_ = registry.Register(cmd2)

	treeCommands := registry.GetForView(ViewTaskDetail)
	found := false
	for _, cmd := range treeCommands {
		if cmd.Name == "Start Task" {
			found = true
			msg := cmd.Handler().(resultMsg)
			if msg.view != "tree" {
				t.Errorf("expected tree handler, got %s", msg.view)
			}
		}
	}
	if !found {
		t.Error("expected to find 'Start Task' in project tree view commands")
	}

	outlinerCommands := registry.GetForView(ViewOutliner)
	found = false
	for _, cmd := range outlinerCommands {
		if cmd.Name == "Start Task" {
			found = true
			msg := cmd.Handler().(resultMsg)
			if msg.view != "outliner" {
				t.Errorf("expected outliner handler, got %s", msg.view)
			}
		}
	}
	if !found {
		t.Error("expected to find 'Start Task' in outliner view commands")
	}
}

func TestGetForViewNoDuplicates(t *testing.T) {
	registry := NewCommandRegistry()

	cmd1 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewTaskDetail},
		Handler:  func() tea.Msg { return nil },
	}
	cmd2 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewOutliner},
		Handler:  func() tea.Msg { return nil },
	}
	global := Command{
		Name:     "Quit",
		Category: CategoryNavigation,
		Handler:  func() tea.Msg { return nil },
	}

	_ = registry.Register(cmd1)
	_ = registry.Register(cmd2)
	_ = registry.Register(global)

	treeCommands := registry.GetForView(ViewTaskDetail)
	nameCount := make(map[string]int)
	for _, cmd := range treeCommands {
		nameCount[cmd.Name]++
	}
	for name, count := range nameCount {
		if count > 1 {
			t.Errorf("duplicate command '%s' in GetForView result (count: %d)", name, count)
		}
	}
}

func TestGetAllDeduplicatesSameNameCommands(t *testing.T) {
	registry := NewCommandRegistry()

	cmd1 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewTaskDetail},
		Handler:  func() tea.Msg { return nil },
	}
	cmd2 := Command{
		Name:     "Start Task",
		Category: CategoryTasks,
		Views:    []PresentationID{ViewOutliner},
		Handler:  func() tea.Msg { return nil },
	}
	other := Command{
		Name:     "Quit",
		Category: CategoryNavigation,
		Handler:  func() tea.Msg { return nil },
	}

	_ = registry.Register(cmd1)
	_ = registry.Register(cmd2)
	_ = registry.Register(other)

	all := registry.GetAll()
	if len(all) != 2 {
		t.Errorf("expected GetAll to return 2 deduplicated commands, got %d", len(all))
	}

	nameCount := make(map[string]int)
	for _, cmd := range all {
		nameCount[cmd.Name]++
	}
	for name, count := range nameCount {
		if count > 1 {
			t.Errorf("duplicate command '%s' in GetAll result (count: %d)", name, count)
		}
	}
}

func TestPreferredVariantSelectionSkipsUnavailableCommands(t *testing.T) {
	registry := NewCommandRegistry()
	treeAvailable := false
	outlinerAvailable := true
	if err := registry.Register(Command{
		Name: "Scoped", Shortcut: "tree", Category: CategoryTasks,
		Views: []PresentationID{ViewTaskDetail}, Available: func() bool { return treeAvailable },
	}); err != nil {
		t.Fatalf("Register tree variant: %v", err)
	}
	if err := registry.Register(Command{
		Name: "Scoped", Shortcut: "outliner", Category: CategoryTasks,
		Views: []PresentationID{ViewOutliner}, Available: func() bool { return outlinerAvailable },
	}); err != nil {
		t.Fatalf("Register outliner variant: %v", err)
	}

	assertPreferred := func(label string, cmd Command, ok bool, wantShortcut string) {
		t.Helper()
		if !ok || cmd.Shortcut != wantShortcut {
			t.Fatalf("%s = (%q, %v), want shortcut %q", label, cmd.Shortcut, ok, wantShortcut)
		}
	}

	all := registry.GetAll()
	if len(all) != 1 {
		t.Fatalf("GetAll = %v, want one available variant", commandNames(all))
	}
	assertPreferred("GetAll", all[0], true, "outliner")
	byCategory := registry.GetByCategory(CategoryTasks)
	if len(byCategory) != 1 {
		t.Fatalf("GetByCategory = %v, want one available variant", commandNames(byCategory))
	}
	assertPreferred("GetByCategory", byCategory[0], true, "outliner")
	cmd, ok := registry.GetByName("scoped")
	assertPreferred("GetByName", cmd, ok, "outliner")
	if commands := registry.GetForView(ViewTaskDetail); len(commands) != 0 {
		t.Fatalf("GetForView(ProjectTree) = %v, want unavailable variant omitted", commandNames(commands))
	}
	commands := registry.GetForView(ViewOutliner)
	if len(commands) != 1 {
		t.Fatalf("GetForView(Outliner) = %v, want one command", commandNames(commands))
	}
	assertPreferred("GetForView(Outliner)", commands[0], true, "outliner")

	treeAvailable = true
	outlinerAvailable = false
	cmd, ok = registry.GetByName("Scoped")
	assertPreferred("GetByName after predicate change", cmd, ok, "tree")
}

func TestViewsOverlap(t *testing.T) {
	if !viewsOverlap(nil, nil) {
		t.Error("expected two global (empty) view lists to overlap")
	}

	if !viewsOverlap(nil, []PresentationID{ViewTaskDetail}) {
		t.Error("expected global to overlap with specific view")
	}

	if !viewsOverlap([]PresentationID{ViewTaskDetail, ViewOutliner}, []PresentationID{ViewOutliner, ViewDashboard}) {
		t.Error("expected overlapping view lists to overlap")
	}

	if viewsOverlap([]PresentationID{ViewTaskDetail}, []PresentationID{ViewDashboard}) {
		t.Error("expected disjoint view lists to not overlap")
	}
}
