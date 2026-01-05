package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Tests for Register duplicate detection

func TestRegisterDuplicateCommandReturnsError(t *testing.T) {
	registry := NewCommandRegistry()

	firstCmd := Command{
		Name:        "Create Task",
		Description: "Add a new task",
		Category:    CategoryTasks,
		Handler:     func() tea.Msg { return nil },
	}

	err := registry.Register(firstCmd)
	if err != nil {
		t.Fatalf("expected first Register to succeed, got error: %v", err)
	}

	duplicateCmd := Command{
		Name:        "Create Task",
		Description: "Different description",
		Category:    CategoryView,
		Handler:     func() tea.Msg { return nil },
	}

	err = registry.Register(duplicateCmd)
	if err == nil {
		t.Error("expected error when registering duplicate command, got nil")
	}
}

func TestRegisterDuplicateErrorContainsCommandName(t *testing.T) {
	registry := NewCommandRegistry()

	cmdName := "Navigate Home"
	firstCmd := Command{
		Name:        cmdName,
		Description: "Go to home view",
		Category:    CategoryNavigation,
		Handler:     func() tea.Msg { return nil },
	}

	_ = registry.Register(firstCmd)

	duplicateCmd := Command{
		Name:        cmdName,
		Description: "Different description",
		Category:    CategoryView,
		Handler:     func() tea.Msg { return nil },
	}

	err := registry.Register(duplicateCmd)
	if err == nil {
		t.Fatal("expected error when registering duplicate command")
	}

	if !strings.Contains(err.Error(), cmdName) {
		t.Errorf("expected error message to contain command name '%s', got: %s", cmdName, err.Error())
	}
}

func TestRegisterDuplicatePreservesOriginalCommand(t *testing.T) {
	registry := NewCommandRegistry()

	originalDescription := "Original description"
	firstCmd := Command{
		Name:        "Test Command",
		Description: originalDescription,
		Category:    CategoryTasks,
		Handler:     func() tea.Msg { return nil },
	}

	_ = registry.Register(firstCmd)

	duplicateCmd := Command{
		Name:        "Test Command",
		Description: "Replacement description",
		Category:    CategoryView,
		Handler:     func() tea.Msg { return nil },
	}

	_ = registry.Register(duplicateCmd)

	cmd, found := registry.GetByName("Test Command")
	if !found {
		t.Fatal("expected command to still exist after duplicate registration attempt")
	}

	if cmd.Description != originalDescription {
		t.Errorf("expected original description '%s' to be preserved, got '%s'", originalDescription, cmd.Description)
	}
}

func TestRegisterMultipleUniqueCommandsSucceeds(t *testing.T) {
	registry := NewCommandRegistry()

	commands := []Command{
		{Name: "Command A", Description: "First", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Command B", Description: "Second", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Command C", Description: "Third", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	for _, cmd := range commands {
		err := registry.Register(cmd)
		if err != nil {
			t.Errorf("expected Register to succeed for '%s', got error: %v", cmd.Name, err)
		}
	}

	if len(registry.GetAll()) != 3 {
		t.Errorf("expected 3 commands registered, got %d", len(registry.GetAll()))
	}
}

// Tests for RegisterBatch duplicate detection

func TestRegisterBatchWithDuplicatesReturnsError(t *testing.T) {
	registry := NewCommandRegistry()

	cmds := []Command{
		{Name: "Unique A", Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Duplicate", Description: "First occurrence", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Duplicate", Description: "Second occurrence", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
		{Name: "Unique B", Description: "Fourth command", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
	}

	err := registry.RegisterBatch(cmds)
	if err == nil {
		t.Error("expected error when registering batch with duplicates, got nil")
	}
}

func TestRegisterBatchDuplicateErrorContainsDuplicateName(t *testing.T) {
	registry := NewCommandRegistry()

	duplicateName := "Special Command"
	cmds := []Command{
		{Name: "First", Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: duplicateName, Description: "First occurrence", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: duplicateName, Description: "Second occurrence", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	err := registry.RegisterBatch(cmds)
	if err == nil {
		t.Fatal("expected error when registering batch with duplicates")
	}

	if !strings.Contains(err.Error(), duplicateName) {
		t.Errorf("expected error message to contain duplicate name '%s', got: %s", duplicateName, err.Error())
	}
}

func TestRegisterBatchPartialRegistrationOnDuplicate(t *testing.T) {
	registry := NewCommandRegistry()

	cmds := []Command{
		{Name: "First Command", Description: "Registered before duplicate", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Second Command", Description: "Also registered before duplicate", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "First Command", Description: "Duplicate of first", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
		{Name: "Fourth Command", Description: "Should not be registered", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	allCommands := registry.GetAll()
	if len(allCommands) != 2 {
		t.Errorf("expected 2 commands registered before duplicate, got %d", len(allCommands))
	}

	if _, found := registry.GetByName("First Command"); !found {
		t.Error("expected 'First Command' to be registered")
	}
	if _, found := registry.GetByName("Second Command"); !found {
		t.Error("expected 'Second Command' to be registered")
	}
}

func TestRegisterBatchCommandsAfterDuplicateNotRegistered(t *testing.T) {
	registry := NewCommandRegistry()

	cmds := []Command{
		{Name: "Before", Description: "Registered", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Before", Description: "Duplicate", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "After Duplicate", Description: "Should not be registered", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	if _, found := registry.GetByName("After Duplicate"); found {
		t.Error("expected 'After Duplicate' to NOT be registered after duplicate error")
	}
}

func TestRegisterBatchWithPreExistingDuplicateReturnsError(t *testing.T) {
	registry := NewCommandRegistry()

	existingCmd := Command{
		Name:        "Existing Command",
		Description: "Already registered",
		Category:    CategoryTasks,
		Handler:     func() tea.Msg { return nil },
	}
	_ = registry.Register(existingCmd)

	cmds := []Command{
		{Name: "New Command", Description: "New", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Existing Command", Description: "Duplicate of pre-existing", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	err := registry.RegisterBatch(cmds)
	if err == nil {
		t.Error("expected error when batch contains duplicate of pre-existing command")
	}

	if !strings.Contains(err.Error(), "Existing Command") {
		t.Errorf("expected error to contain 'Existing Command', got: %s", err.Error())
	}
}

func TestRegisterBatchEmptySliceSucceeds(t *testing.T) {
	registry := NewCommandRegistry()

	err := registry.RegisterBatch([]Command{})
	if err != nil {
		t.Errorf("expected RegisterBatch with empty slice to succeed, got error: %v", err)
	}

	if len(registry.GetAll()) != 0 {
		t.Errorf("expected registry to remain empty, got %d commands", len(registry.GetAll()))
	}
}

func TestRegisterBatchFirstCommandDuplicateReturnsErrorImmediately(t *testing.T) {
	registry := NewCommandRegistry()

	existingCmd := Command{
		Name:        "Preregistered",
		Description: "Already in registry",
		Category:    CategoryTasks,
		Handler:     func() tea.Msg { return nil },
	}
	_ = registry.Register(existingCmd)

	cmds := []Command{
		{Name: "Preregistered", Description: "Duplicate is first in batch", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "After Dup", Description: "Should not be registered", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	err := registry.RegisterBatch(cmds)
	if err == nil {
		t.Error("expected error when first command in batch is duplicate")
	}

	if len(registry.GetAll()) != 1 {
		t.Errorf("expected only pre-existing command in registry, got %d", len(registry.GetAll()))
	}

	if _, found := registry.GetByName("After Dup"); found {
		t.Error("expected 'After Dup' to NOT be registered")
	}
}

// Tests for GetAll query method

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

// Tests for GetByCategory query method

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

// Tests for GetByName query method

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
