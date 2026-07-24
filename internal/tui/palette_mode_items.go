package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// outlinerModeCommands derives registered commands for non-Work catalog modes.
// Work modes are intentionally Tab-only; the command registry remains the
// source for command-palette results and scenario command execution.
func outlinerModeCommands() []Command {
	catalog := OutlinerModeCatalog()
	commands := make([]Command, 0, len(catalog))
	for _, mode := range catalog {
		if mode.Category == ModeCategoryWork {
			continue
		}
		commands = append(commands, outlinerModeCommand(mode))
	}
	return commands
}

// outlinerModeCommand builds the palette command for a single catalog mode.
// Executing it retains the canonical app-scoped Outliner and dispatches the mode
// through SetMode, which ApplyOpenOptions routes to selectOutlinerMode. The
// command is searchable by the mode's label (its Name), its stable ID, and every
// catalog alias (its Aliases).
func outlinerModeCommand(mode OutlinerMode) Command {
	id := mode.ID
	return Command{
		Name:         mode.Label,
		Description:  "Switch to the " + mode.Label + " Outliner mode",
		Category:     CategoryViews,
		Aliases:      outlinerModeCommandAliases(mode),
		Reachability: CommandReachRouteMode,
		FinalTarget:  "TaskWorkspace/Mode/" + string(mode.ID),
		Handler: func() tea.Msg {
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: id}}
		},
	}
}

// outlinerModeCommandAliases returns the mode's stable ID followed by every
// catalog alias, de-duplicated case-insensitively so palette search covers the
// ID and aliases without redundant terms. The label is already searched through
// the command Name, so a catalog alias equal to the label collapses away.
func outlinerModeCommandAliases(mode OutlinerMode) []string {
	seen := map[string]bool{
		strings.ToLower(strings.TrimSpace(mode.Label)): true,
	}
	aliases := make([]string, 0, len(mode.Aliases)+1)
	add := func(s string) {
		key := strings.ToLower(strings.TrimSpace(s))
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		aliases = append(aliases, s)
	}
	add(string(mode.ID))
	for _, alias := range mode.Aliases {
		add(alias)
	}
	return aliases
}
