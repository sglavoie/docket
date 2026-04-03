package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestGetAllReturnsExpectedCountAfterRegistrations(t *testing.T) {
	registry := NewCommandRegistry()

	if len(registry.GetAll()) != 0 {
		t.Errorf("expected empty registry to return 0 commands, got %d", len(registry.GetAll()))
	}

	_ = registry.Register(Command{Name: "First", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})
	if len(registry.GetAll()) != 1 {
		t.Errorf("expected 1 command after first registration, got %d", len(registry.GetAll()))
	}

	_ = registry.Register(Command{Name: "Second", Category: CategoryView, Handler: func() tea.Msg { return nil }})
	if len(registry.GetAll()) != 2 {
		t.Errorf("expected 2 commands after second registration, got %d", len(registry.GetAll()))
	}

	_ = registry.Register(Command{Name: "Third", Category: CategoryHelp, Handler: func() tea.Msg { return nil }})
	_ = registry.Register(Command{Name: "Fourth", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }})
	_ = registry.Register(Command{Name: "Fifth", Category: CategoryProjects, Handler: func() tea.Msg { return nil }})
	if len(registry.GetAll()) != 5 {
		t.Errorf("expected 5 commands after five registrations, got %d", len(registry.GetAll()))
	}
}

func TestGetAllReturnsEmptySliceForEmptyRegistry(t *testing.T) {
	registry := NewCommandRegistry()

	result := registry.GetAll()
	if result == nil {
		t.Error("expected GetAll to return empty slice, not nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(result))
	}
}

func TestGetAllReturnsAllRegisteredCommands(t *testing.T) {
	registry := NewCommandRegistry()

	commands := []Command{
		{Name: "Alpha", Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Beta", Description: "Second command", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Gamma", Description: "Third command", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	for _, cmd := range commands {
		_ = registry.Register(cmd)
	}

	result := registry.GetAll()
	if len(result) != len(commands) {
		t.Fatalf("expected %d commands, got %d", len(commands), len(result))
	}

	nameSet := make(map[string]bool)
	for _, cmd := range result {
		nameSet[cmd.Name] = true
	}

	for _, cmd := range commands {
		if !nameSet[cmd.Name] {
			t.Errorf("expected command '%s' to be in result", cmd.Name)
		}
	}
}

func TestGetByCategoryFiltersToExpectedSubset(t *testing.T) {
	registry := NewCommandRegistry()

	commands := []Command{
		{Name: "Task 1", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Task 2", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Task 3", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "View 1", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "View 2", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Help 1", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
		{Name: "Nav 1", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
	}

	for _, cmd := range commands {
		_ = registry.Register(cmd)
	}

	taskCommands := registry.GetByCategory(CategoryTasks)
	if len(taskCommands) != 3 {
		t.Errorf("expected 3 task commands, got %d", len(taskCommands))
	}

	viewCommands := registry.GetByCategory(CategoryView)
	if len(viewCommands) != 2 {
		t.Errorf("expected 2 view commands, got %d", len(viewCommands))
	}

	helpCommands := registry.GetByCategory(CategoryHelp)
	if len(helpCommands) != 1 {
		t.Errorf("expected 1 help command, got %d", len(helpCommands))
	}

	navCommands := registry.GetByCategory(CategoryNavigation)
	if len(navCommands) != 1 {
		t.Errorf("expected 1 navigation command, got %d", len(navCommands))
	}
}

func TestGetByCategoryReturnsOnlyMatchingCategory(t *testing.T) {
	registry := NewCommandRegistry()

	_ = registry.Register(Command{Name: "Task A", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})
	_ = registry.Register(Command{Name: "Task B", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})
	_ = registry.Register(Command{Name: "View A", Category: CategoryView, Handler: func() tea.Msg { return nil }})
	_ = registry.Register(Command{Name: "Help A", Category: CategoryHelp, Handler: func() tea.Msg { return nil }})

	result := registry.GetByCategory(CategoryTasks)

	for _, cmd := range result {
		if cmd.Category != CategoryTasks {
			t.Errorf("expected all returned commands to have category %s, got %s for command '%s'",
				CategoryTasks, cmd.Category, cmd.Name)
		}
	}
}

func TestGetByCategoryReturnsEmptySliceForNoMatches(t *testing.T) {
	registry := NewCommandRegistry()

	_ = registry.Register(Command{Name: "Task A", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})
	_ = registry.Register(Command{Name: "View A", Category: CategoryView, Handler: func() tea.Msg { return nil }})

	result := registry.GetByCategory(CategoryProjects)
	if result == nil {
		t.Error("expected GetByCategory to return empty slice, not nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice for non-matching category, got %d elements", len(result))
	}
}

func TestGetByCategoryReturnsEmptySliceForEmptyRegistry(t *testing.T) {
	registry := NewCommandRegistry()

	result := registry.GetByCategory(CategoryTasks)
	if result == nil {
		t.Error("expected GetByCategory to return empty slice, not nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice for empty registry, got %d elements", len(result))
	}
}

func TestGetByCategoryWithAllCategories(t *testing.T) {
	registry := NewCommandRegistry()

	categories := []CommandCategory{
		CategoryNavigation, CategoryActions, CategoryViews, CategorySearch,
		CategoryTasks, CategoryProjects, CategoryView, CategoryFilters, CategoryHelp,
	}

	for i, cat := range categories {
		_ = registry.Register(Command{
			Name:     "Cmd " + string(cat),
			Category: cat,
			Handler:  func() tea.Msg { return nil },
		})
		if i%2 == 0 {
			_ = registry.Register(Command{
				Name:     "Extra " + string(cat),
				Category: cat,
				Handler:  func() tea.Msg { return nil },
			})
		}
	}

	for i, cat := range categories {
		result := registry.GetByCategory(cat)
		expectedCount := 1
		if i%2 == 0 {
			expectedCount = 2
		}
		if len(result) != expectedCount {
			t.Errorf("expected %d commands for category %s, got %d", expectedCount, cat, len(result))
		}
	}
}

func TestGetByNameWithVariousCasesReturnsSameCommand(t *testing.T) {
	registry := NewCommandRegistry()

	expected := Command{
		Name:        "Create Task",
		Description: "Add a new task to the project",
		Category:    CategoryTasks,
		Handler:     func() tea.Msg { return nil },
	}
	_ = registry.Register(expected)

	testCases := []string{
		"Create Task",
		"create task",
		"CREATE TASK",
		"CrEaTe TaSk",
		"CREATE task",
		"create TASK",
	}

	for _, name := range testCases {
		cmd, found := registry.GetByName(name)
		if !found {
			t.Errorf("expected to find command with name '%s'", name)
			continue
		}
		if cmd.Name != expected.Name {
			t.Errorf("expected command name '%s', got '%s' when searching for '%s'",
				expected.Name, cmd.Name, name)
		}
		if cmd.Description != expected.Description {
			t.Errorf("expected description '%s', got '%s' when searching for '%s'",
				expected.Description, cmd.Description, name)
		}
		if cmd.Category != expected.Category {
			t.Errorf("expected category '%s', got '%s' when searching for '%s'",
				expected.Category, cmd.Category, name)
		}
	}
}

func TestGetByNameWithMissingNameReturnsNotFound(t *testing.T) {
	registry := NewCommandRegistry()

	_ = registry.Register(Command{Name: "Existing Command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})

	cmd, found := registry.GetByName("Non-Existent Command")
	if found {
		t.Error("expected found to be false for missing command")
	}
	if cmd.Name != "" {
		t.Errorf("expected empty command name for missing command, got '%s'", cmd.Name)
	}
}

func TestGetByNameReturnsNotFoundForEmptyRegistry(t *testing.T) {
	registry := NewCommandRegistry()

	cmd, found := registry.GetByName("Any Command")
	if found {
		t.Error("expected found to be false for empty registry")
	}
	if cmd.Name != "" {
		t.Errorf("expected empty command name, got '%s'", cmd.Name)
	}
}

func TestGetByNameWithEmptyStringReturnsNotFound(t *testing.T) {
	registry := NewCommandRegistry()

	_ = registry.Register(Command{Name: "Test Command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})

	cmd, found := registry.GetByName("")
	if found {
		t.Error("expected found to be false for empty string search")
	}
	if cmd.Name != "" {
		t.Errorf("expected empty command name, got '%s'", cmd.Name)
	}
}

func TestGetByNameWithSimilarNamesReturnsExactMatch(t *testing.T) {
	registry := NewCommandRegistry()

	_ = registry.Register(Command{Name: "Create", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})
	_ = registry.Register(Command{Name: "Create Task", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})
	_ = registry.Register(Command{Name: "Create Project", Category: CategoryProjects, Handler: func() tea.Msg { return nil }})

	cmd, found := registry.GetByName("create task")
	if !found {
		t.Fatal("expected to find 'Create Task'")
	}
	if cmd.Name != "Create Task" {
		t.Errorf("expected 'Create Task', got '%s'", cmd.Name)
	}

	cmd, found = registry.GetByName("CREATE")
	if !found {
		t.Fatal("expected to find 'Create'")
	}
	if cmd.Name != "Create" {
		t.Errorf("expected 'Create', got '%s'", cmd.Name)
	}
}

func TestGetByNameWithWhitespaceVariationsDoesNotMatch(t *testing.T) {
	registry := NewCommandRegistry()

	_ = registry.Register(Command{Name: "Create Task", Category: CategoryTasks, Handler: func() tea.Msg { return nil }})

	testCases := []string{
		" Create Task",
		"Create Task ",
		"Create  Task",
		"CreateTask",
	}

	for _, name := range testCases {
		_, found := registry.GetByName(name)
		if found {
			t.Errorf("expected '%s' to NOT match 'Create Task' (whitespace should be exact)", name)
		}
	}
}

func TestGetByNamePreservesOriginalCommandData(t *testing.T) {
	registry := NewCommandRegistry()

	original := Command{
		Name:        "Test Command",
		Description: "Original description",
		Shortcut:    "ctrl+t",
		Category:    CategoryTasks,
		Handler:     func() tea.Msg { return nil },
	}
	_ = registry.Register(original)

	cmd, found := registry.GetByName("test command")
	if !found {
		t.Fatal("expected to find command")
	}

	if cmd.Name != original.Name {
		t.Errorf("expected Name '%s', got '%s'", original.Name, cmd.Name)
	}
	if cmd.Description != original.Description {
		t.Errorf("expected Description '%s', got '%s'", original.Description, cmd.Description)
	}
	if cmd.Shortcut != original.Shortcut {
		t.Errorf("expected Shortcut '%s', got '%s'", original.Shortcut, cmd.Shortcut)
	}
	if cmd.Category != original.Category {
		t.Errorf("expected Category '%s', got '%s'", original.Category, cmd.Category)
	}
}