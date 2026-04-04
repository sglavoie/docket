package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

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
