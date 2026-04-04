package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

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
