package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
)

func TestCommandStruct(t *testing.T) {
	handlerCalled := false
	testHandler := func() tea.Msg {
		handlerCalled = true
		return nil
	}

	cmd := Command{
		Name:        "Create Task",
		Description: "Add a new task to the current project",
		Shortcut:    "n",
		Category:    CategoryTasks,
		Handler:     testHandler,
	}

	if cmd.Name != "Create Task" {
		t.Errorf("expected Name to be 'Create Task', got '%s'", cmd.Name)
	}

	if cmd.Description != "Add a new task to the current project" {
		t.Errorf("expected Description to be 'Add a new task to the current project', got '%s'", cmd.Description)
	}

	if cmd.Shortcut != "n" {
		t.Errorf("expected Shortcut to be 'n', got '%s'", cmd.Shortcut)
	}

	if cmd.Category != CategoryTasks {
		t.Errorf("expected Category to be CategoryTasks, got '%s'", cmd.Category)
	}

	if cmd.Handler == nil {
		t.Error("expected Handler to be non-nil")
	}

	cmd.Handler()
	if !handlerCalled {
		t.Error("expected Handler to be callable")
	}
}

func TestCommandCategories(t *testing.T) {
	categories := []CommandCategory{
		CategoryNavigation,
		CategoryTasks,
		CategoryProjects,
		CategoryView,
		CategoryFilters,
		CategoryHelp,
	}

	expected := []string{
		"Navigation",
		"Tasks",
		"Projects",
		"View",
		"Filters",
		"Help",
	}

	for i, cat := range categories {
		if string(cat) != expected[i] {
			t.Errorf("expected category %d to be '%s', got '%s'", i, expected[i], cat)
		}
	}
}

func TestCommandWithNoShortcut(t *testing.T) {
	cmd := Command{
		Name:        "Some Action",
		Description: "An action without a keyboard shortcut",
		Shortcut:    "",
		Category:    CategoryView,
		Handler:     func() tea.Msg { return nil },
	}

	_ = cmd.Name
	_ = cmd.Description
	_ = cmd.Category
	_ = cmd.Handler

	if cmd.Shortcut != "" {
		t.Errorf("expected empty Shortcut, got '%s'", cmd.Shortcut)
	}
}

func TestCommandHandlerReturnsMessage(t *testing.T) {
	type TestMsg struct {
		Value string
	}

	cmd := Command{
		Name:        "Test Command",
		Description: "Returns a test message",
		Shortcut:    "t",
		Category:    CategoryHelp,
		Handler: func() tea.Msg {
			return TestMsg{Value: "hello"}
		},
	}

	_ = cmd.Name
	_ = cmd.Description
	_ = cmd.Shortcut
	_ = cmd.Category

	result := cmd.Handler()
	msg, ok := result.(TestMsg)
	if !ok {
		t.Error("expected Handler to return TestMsg")
	}
	if msg.Value != "hello" {
		t.Errorf("expected message Value to be 'hello', got '%s'", msg.Value)
	}
}

func TestNewCommandRegistryReturnsNonNilPointer(t *testing.T) {
	registry := NewCommandRegistry()
	if registry == nil {
		t.Error("expected NewCommandRegistry to return non-nil pointer")
	}
}

func TestNewCommandRegistryHasInitializedEmptyMap(t *testing.T) {
	registry := NewCommandRegistry()
	if registry.commands == nil {
		t.Error("expected internal map to be initialized, got nil")
	}
	if len(registry.commands) != 0 {
		t.Errorf("expected internal map to be empty, got %d entries", len(registry.commands))
	}
}

func TestNewCommandRegistryGetAllReturnsEmptySlice(t *testing.T) {
	registry := NewCommandRegistry()
	commands := registry.GetAll()
	if commands == nil {
		t.Error("expected GetAll to return non-nil slice")
	}
	if len(commands) != 0 {
		t.Errorf("expected GetAll to return empty slice, got %d commands", len(commands))
	}
}

func TestRegisterBatchWithThreeUniqueCommandsAddsAll(t *testing.T) {
	registry := NewCommandRegistry()
	cmds := []Command{
		{Name: "Command A", Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Command B", Description: "Second command", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Command C", Description: "Third command", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	err := registry.RegisterBatch(cmds)
	if err != nil {
		t.Errorf("expected no error when registering unique commands, got %v", err)
	}

	allCommands := registry.GetAll()
	if len(allCommands) != 3 {
		t.Errorf("expected 3 commands to be registered, got %d", len(allCommands))
	}

	for _, cmd := range cmds {
		found := false
		for _, registered := range allCommands {
			if registered.Name == cmd.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected command '%s' to be registered", cmd.Name)
		}
	}
}

func TestRegisterBatchWithDuplicateInMiddleReturnsErrorAndPreservesEarlier(t *testing.T) {
	registry := NewCommandRegistry()
	cmds := []Command{
		{Name: "Command A", Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Command A", Description: "Duplicate command", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Command C", Description: "Third command", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	err := registry.RegisterBatch(cmds)
	if err == nil {
		t.Error("expected error when registering duplicate command")
	}

	allCommands := registry.GetAll()
	if len(allCommands) != 1 {
		t.Errorf("expected 1 command to be registered before duplicate, got %d", len(allCommands))
	}

	if allCommands[0].Name != "Command A" {
		t.Errorf("expected 'Command A' to be registered, got '%s'", allCommands[0].Name)
	}
	if allCommands[0].Description != "First command" {
		t.Errorf("expected description 'First command', got '%s'", allCommands[0].Description)
	}
}

func TestRegisterBatchCommandsAfterDuplicateAreNotRegistered(t *testing.T) {
	registry := NewCommandRegistry()
	cmds := []Command{
		{Name: "Command A", Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Command A", Description: "Duplicate command", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Command C", Description: "Third command", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	allCommands := registry.GetAll()
	for _, cmd := range allCommands {
		if cmd.Name == "Command C" {
			t.Error("expected 'Command C' (after duplicate) to NOT be registered")
		}
	}
}

func TestGetAllReturnsSliceWithLengthMatchingRegisteredCommands(t *testing.T) {
	registry := NewCommandRegistry()
	cmds := []Command{
		{Name: "Command A", Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Command B", Description: "Second command", Category: CategoryView, Handler: func() tea.Msg { return nil }},
		{Name: "Command C", Description: "Third command", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
		{Name: "Command D", Description: "Fourth command", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	allCommands := registry.GetAll()
	if len(allCommands) != 4 {
		t.Errorf("expected GetAll to return 4 commands, got %d", len(allCommands))
	}
}

func TestGetAllReturnedSliceModificationDoesNotAffectRegistry(t *testing.T) {
	registry := NewCommandRegistry()
	cmds := []Command{
		{Name: "Command A", Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Command B", Description: "Second command", Category: CategoryView, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	returned := registry.GetAll()
	originalLen := len(returned)

	// Modify returned slice (should not affect original registry)
	_ = append(returned, Command{Name: "Command C", Description: "Appended command", Category: CategoryHelp, Handler: func() tea.Msg { return nil }})

	freshGet := registry.GetAll()
	if len(freshGet) != originalLen {
		t.Errorf("expected registry to still have %d commands after modifying returned slice, got %d", originalLen, len(freshGet))
	}

	for _, cmd := range freshGet {
		if cmd.Name == "Command C" {
			t.Error("appending to returned slice should not add command to registry")
		}
	}
}

func TestGetAllReturnedSliceElementModificationDoesNotAffectRegistry(t *testing.T) {
	registry := NewCommandRegistry()
	originalName := "Original Name"
	cmds := []Command{
		{Name: originalName, Description: "First command", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	returned := registry.GetAll()
	returned[0].Name = "Modified Name"

	freshGet := registry.GetAll()
	if freshGet[0].Name != originalName {
		t.Errorf("expected registry command name to remain '%s', got '%s'", originalName, freshGet[0].Name)
	}
}

func TestGetByCategoryReturnsOnlyMatchingCommands(t *testing.T) {
	registry := NewCommandRegistry()
	cmds := []Command{
		{Name: "Navigate Up", Description: "Move cursor up", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
		{Name: "Navigate Down", Description: "Move cursor down", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
		{Name: "Create Task", Description: "Add a new task", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Delete Task", Description: "Remove a task", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Show Help", Description: "Display help", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	navCommands := registry.GetByCategory(CategoryNavigation)

	for _, cmd := range navCommands {
		if cmd.Category != CategoryNavigation {
			t.Errorf("expected all commands to have CategoryNavigation, got '%s' for command '%s'", cmd.Category, cmd.Name)
		}
	}

	expectedNames := map[string]bool{"Navigate Up": true, "Navigate Down": true}
	for _, cmd := range navCommands {
		if !expectedNames[cmd.Name] {
			t.Errorf("unexpected command '%s' in CategoryNavigation results", cmd.Name)
		}
	}
}

func TestGetByCategoryWithUnusedCategoryReturnsEmptySlice(t *testing.T) {
	registry := NewCommandRegistry()
	cmds := []Command{
		{Name: "Create Task", Description: "Add a new task", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Delete Task", Description: "Remove a task", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Show Help", Description: "Display help", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	searchCommands := registry.GetByCategory(CategorySearch)

	if searchCommands == nil {
		t.Error("expected GetByCategory to return non-nil slice for unused category")
	}
	if len(searchCommands) != 0 {
		t.Errorf("expected GetByCategory to return empty slice for unused category, got %d commands", len(searchCommands))
	}
}

func TestGetByCategoryReturnedLengthMatchesExpectedCount(t *testing.T) {
	registry := NewCommandRegistry()
	cmds := []Command{
		{Name: "Nav 1", Description: "Navigation command 1", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
		{Name: "Nav 2", Description: "Navigation command 2", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
		{Name: "Nav 3", Description: "Navigation command 3", Category: CategoryNavigation, Handler: func() tea.Msg { return nil }},
		{Name: "Task 1", Description: "Task command 1", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Task 2", Description: "Task command 2", Category: CategoryTasks, Handler: func() tea.Msg { return nil }},
		{Name: "Help 1", Description: "Help command", Category: CategoryHelp, Handler: func() tea.Msg { return nil }},
	}

	_ = registry.RegisterBatch(cmds)

	navCommands := registry.GetByCategory(CategoryNavigation)
	if len(navCommands) != 3 {
		t.Errorf("expected 3 navigation commands, got %d", len(navCommands))
	}

	taskCommands := registry.GetByCategory(CategoryTasks)
	if len(taskCommands) != 2 {
		t.Errorf("expected 2 task commands, got %d", len(taskCommands))
	}

	helpCommands := registry.GetByCategory(CategoryHelp)
	if len(helpCommands) != 1 {
		t.Errorf("expected 1 help command, got %d", len(helpCommands))
	}
}

func TestNewAppModelRegistersGlobalCommands(t *testing.T) {
	cfg := &config.Config{}
	manager := &config.Manager{}

	app := NewAppModel(cfg, manager)
	registry := app.GetCommandRegistry()

	// Verify help command with shortcut "?"
	helpCmd, found := registry.GetByName("help")
	if !found {
		t.Fatal("expected 'help' command to be registered")
	}
	if helpCmd.Shortcut != "?" {
		t.Errorf("expected help command shortcut to be '?', got '%s'", helpCmd.Shortcut)
	}
	if helpCmd.Handler == nil {
		t.Error("expected help command to have a handler")
	}

	// Verify quit command with shortcut "ctrl+c"
	quitCmd, found := registry.GetByName("quit")
	if !found {
		t.Fatal("expected 'quit' command to be registered")
	}
	if quitCmd.Shortcut != "ctrl+c" {
		t.Errorf("expected quit command shortcut to be 'ctrl+c', got '%s'", quitCmd.Shortcut)
	}
	if quitCmd.Handler == nil {
		t.Error("expected quit command to have a handler")
	}

	// Verify global search command with shortcut "/"
	searchCmd, found := registry.GetByName("global search")
	if !found {
		t.Fatal("expected 'global search' command to be registered")
	}
	if searchCmd.Shortcut != ";/" {
		t.Errorf("expected global search command shortcut to be ';/', got '%s'", searchCmd.Shortcut)
	}
	if searchCmd.Handler == nil {
		t.Error("expected global search command to have a handler")
	}

	// Verify GetAll returns at least 3 commands
	allCommands := registry.GetAll()
	if len(allCommands) < 3 {
		t.Errorf("expected at least 3 commands to be registered, got %d", len(allCommands))
	}
}

func TestGlobalCommandHandlersReturnCorrectMessages(t *testing.T) {
	cfg := &config.Config{}
	manager := &config.Manager{}

	app := NewAppModel(cfg, manager)
	registry := app.GetCommandRegistry()

	// Test help command handler returns ShowHelpMsg
	helpCmd, _ := registry.GetByName("help")
	helpMsg := helpCmd.Handler()
	if _, ok := helpMsg.(ShowHelpMsg); !ok {
		t.Errorf("expected help command handler to return ShowHelpMsg, got %T", helpMsg)
	}

	// Test quit command handler returns QuitMsg
	quitCmd, _ := registry.GetByName("quit")
	quitMsg := quitCmd.Handler()
	if _, ok := quitMsg.(QuitMsg); !ok {
		t.Errorf("expected quit command handler to return QuitMsg, got %T", quitMsg)
	}

	// Test global search command handler returns ShowGlobalSearchMsg
	searchCmd, _ := registry.GetByName("global search")
	searchMsg := searchCmd.Handler()
	if _, ok := searchMsg.(ShowGlobalSearchMsg); !ok {
		t.Errorf("expected global search command handler to return ShowGlobalSearchMsg, got %T", searchMsg)
	}
}
