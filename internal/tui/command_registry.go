package tui

import (
	"fmt"
	"strings"
)

// CommandRegistry manages command instances and provides centralized command lookup.
// Commands are keyed by their name for efficient retrieval and deduplication.
type CommandRegistry struct {
	commands map[string]Command
}

// Register adds a command to the registry.
// Returns an error if a command with the same name already exists.
func (r *CommandRegistry) Register(cmd Command) error {
	if _, exists := r.commands[cmd.Name]; exists {
		return fmt.Errorf("command already registered: %s", cmd.Name)
	}
	r.commands[cmd.Name] = cmd
	return nil
}

// NewCommandRegistry creates a new command registry with an initialized empty map.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]Command),
	}
}

// RegisterBatch adds multiple commands to the registry.
// Stops on first error and returns it. Commands registered before the error remain registered.
func (r *CommandRegistry) RegisterBatch(cmds []Command) error {
	for _, cmd := range cmds {
		if err := r.Register(cmd); err != nil {
			return err
		}
	}
	return nil
}

// GetAll returns all registered commands as a slice.
// Returns an empty slice if no commands are registered.
func (r *CommandRegistry) GetAll() []Command {
	if len(r.commands) == 0 {
		return []Command{}
	}
	result := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}
	return result
}

// GetByCategory returns all commands matching the specified category.
// Returns an empty slice if no commands match the category.
func (r *CommandRegistry) GetByCategory(category CommandCategory) []Command {
	result := []Command{}
	for _, cmd := range r.commands {
		if cmd.Category == category {
			result = append(result, cmd)
		}
	}
	return result
}

// GetByName returns a command by name using case-insensitive lookup.
// Returns the command and true if found, or a zero Command and false if not found.
func (r *CommandRegistry) GetByName(name string) (Command, bool) {
	nameLower := strings.ToLower(name)
	for _, cmd := range r.commands {
		if strings.ToLower(cmd.Name) == nameLower {
			return cmd, true
		}
	}
	return Command{}, false
}
