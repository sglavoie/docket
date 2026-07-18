package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// CommandRegistry manages command instances and provides centralized command lookup.
// Commands are keyed by their name. Multiple commands with the same name can coexist
// if they have non-overlapping view scopes (e.g., one for ProjectTree and one for Outliner).
type CommandRegistry struct {
	commands map[string][]Command
}

// NewCommandRegistry creates a new command registry with an initialized empty map.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string][]Command),
	}
}

// viewsOverlap returns true if two view slices have any overlap.
// An empty Views slice means "global" (all views), which overlaps with everything.
func viewsOverlap(views1, views2 []ViewType) bool {
	if len(views1) == 0 || len(views2) == 0 {
		return true
	}
	for _, v1 := range views1 {
		if slices.Contains(views2, v1) {
			return true
		}
	}
	return false
}

// Register adds a command to the registry.
// Returns an error if a command with the same name and overlapping view scope already exists.
func (r *CommandRegistry) Register(cmd Command) error {
	if r.commands == nil {
		r.commands = make(map[string][]Command)
	}

	key := cmd.Name
	existing := r.commands[key]
	for existingName, entries := range r.commands {
		if strings.EqualFold(existingName, cmd.Name) {
			key = existingName
			existing = entries
			break
		}
	}
	for _, e := range existing {
		if viewsOverlap(e.Views, cmd.Views) {
			return fmt.Errorf("command already registered: %s", cmd.Name)
		}
	}
	r.commands[key] = append(existing, cmd)
	return nil
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

// GetAll returns all currently available commands as a slice, deduplicated by name.
// When multiple entries share a name, the global one (empty Views) is preferred;
// otherwise the first entry is returned.
// Returns an empty slice if no commands are registered.
func (r *CommandRegistry) GetAll() []Command {
	if len(r.commands) == 0 {
		return []Command{}
	}
	result := make([]Command, 0, len(r.commands))
	for _, entries := range r.commands {
		if cmd, ok := pickPreferred(entries); ok {
			result = append(result, cmd)
		}
	}
	sortCommands(result)
	return result
}

// GetByCategory returns all currently available commands matching the specified
// category, deduplicated by name.
// Returns an empty slice if no commands match the category.
func (r *CommandRegistry) GetByCategory(category CommandCategory) []Command {
	result := []Command{}
	for _, entries := range r.commands {
		cmd, ok := pickPreferred(entries)
		if ok && cmd.Category == category {
			result = append(result, cmd)
		}
	}
	sortCommands(result)
	return result
}

// GetByName returns a currently available command by name using case-insensitive lookup.
// Returns the command and true if found, or a zero Command and false if not found.
// When multiple entries share a name, the global one (empty Views) is preferred.
func (r *CommandRegistry) GetByName(name string) (Command, bool) {
	nameLower := strings.ToLower(name)
	for cmdName, entries := range r.commands {
		if strings.ToLower(cmdName) == nameLower {
			return pickPreferred(entries)
		}
	}
	return Command{}, false
}

// GetForView returns all commands available for the specified view.
// Returns global commands (those with empty Views slice) plus commands
// that explicitly include the given view in their Views slice.
// When multiple entries share a name, only the most specific match for the
// requested view is returned (view-specific beats global).
func (r *CommandRegistry) GetForView(view ViewType) []Command {
	result := []Command{}
	for _, entries := range r.commands {
		var globalCmd *Command
		var viewCmd *Command
		for i := range entries {
			if !commandAvailable(entries[i]) {
				continue
			}
			if len(entries[i].Views) == 0 {
				globalCmd = &entries[i]
			} else {
				if slices.Contains(entries[i].Views, view) {
					viewCmd = &entries[i]
				}
			}
		}
		// Prefer view-specific over global
		if viewCmd != nil {
			result = append(result, *viewCmd)
		} else if globalCmd != nil {
			result = append(result, *globalCmd)
		}
	}
	sortCommands(result)
	return result
}

func commandAvailable(cmd Command) bool {
	return cmd.Available == nil || cmd.Available()
}

func sortCommands(commands []Command) {
	sort.SliceStable(commands, func(i, j int) bool {
		left := strings.ToLower(commands[i].Name)
		right := strings.ToLower(commands[j].Name)
		if left != right {
			return left < right
		}
		if commands[i].Shortcut != commands[j].Shortcut {
			return commands[i].Shortcut < commands[j].Shortcut
		}
		return commands[i].Description < commands[j].Description
	})
}

// pickPreferred returns the preferred currently available command from a list
// of same-named entries. Global commands (empty Views) are preferred; otherwise
// the first available entry is returned.
func pickPreferred(entries []Command) (Command, bool) {
	var firstAvailable *Command
	for i := range entries {
		if !commandAvailable(entries[i]) {
			continue
		}
		if len(entries[i].Views) == 0 {
			return entries[i], true
		}
		if firstAvailable == nil {
			firstAvailable = &entries[i]
		}
	}
	if firstAvailable != nil {
		return *firstAvailable, true
	}
	return Command{}, false
}
