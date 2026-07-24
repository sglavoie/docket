package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// outlinerModePaletteItems derives one selectable palette command for every
// OutlinerModeCatalog entry. It is the single source for palette mode actions:
// the palette no longer hand-maintains a parallel subset (Today/Next/Backlog),
// so adding a catalog entry surfaces a new palette action automatically without
// another registry edit.
func outlinerModePaletteItems() []PaletteItem {
	catalog := OutlinerModeCatalog()
	items := make([]PaletteItem, 0, len(catalog))
	for _, mode := range catalog {
		cmd := outlinerModeCommand(mode)
		items = append(items, PaletteItem{
			Name:        cmd.Name,
			Description: cmd.Description,
			Category:    ItemCategoryCommands,
			Command:     &cmd,
		})
	}
	return items
}

// outlinerModeCommand builds the palette command for a single catalog mode.
// Executing it retains the canonical app-scoped Outliner and dispatches the mode
// through SetMode, which ApplyOpenOptions routes to selectOutlinerMode. The
// command is searchable by the mode's label (its Name), its stable ID, and every
// catalog alias (its Aliases).
func outlinerModeCommand(mode OutlinerMode) Command {
	id := mode.ID
	return Command{
		Name:        mode.Label,
		Description: "Switch to the " + mode.Label + " Outliner mode",
		Category:    CategoryViews,
		Aliases:     outlinerModeCommandAliases(mode),
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
	seen := make(map[string]bool)
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
