package tui

import tea "charm.land/bubbletea/v2"

// FormTabNavigable is implemented by form creators that support tab/shift+tab
// field cycling and [/] tab switching with per-tab focus scoping.
type FormTabNavigable interface {
	blurCurrentField()
	focusCurrentField() tea.Cmd
	getFirstFocusIndexForTab(tab int) int
	getFocusCountForTab(tab int) int
	getTabForFocusIndex(idx int) int
}

// formTabState holds the mutable state shared by form tab navigation helpers.
// Callers pass pointers to their own fields.
type formTabState struct {
	activeTab    *int
	focusIndex   *int
	scrollOffset *int
}

// handleFormTabKey handles tab/shift+tab cycling within the current tab.
// Returns the tea.Cmd from focusing the new field.
func handleFormTabKey(f FormTabNavigable, s formTabState, forward bool) tea.Cmd {
	f.blurCurrentField()

	tabStart := f.getFirstFocusIndexForTab(*s.activeTab)
	tabCount := f.getFocusCountForTab(*s.activeTab)

	if tabCount == 0 {
		return f.focusCurrentField()
	}

	relIdx := max(*s.focusIndex-tabStart, 0)

	if forward {
		relIdx = (relIdx + 1) % tabCount
	} else {
		relIdx = (relIdx + tabCount - 1) % tabCount
	}

	*s.focusIndex = tabStart + relIdx
	*s.activeTab = f.getTabForFocusIndex(*s.focusIndex)

	return f.focusCurrentField()
}

// handleFormTabSwitch handles [/] tab switching. firstTab is the lowest tab
// index (typically 0), lastTab is the highest tab index to cycle through.
func handleFormTabSwitch(f FormTabNavigable, s formTabState, firstTab, lastTab int, forward bool) tea.Cmd {
	f.blurCurrentField()

	if forward {
		if *s.activeTab < lastTab {
			*s.activeTab++
		} else {
			*s.activeTab = firstTab
		}
	} else {
		if *s.activeTab > firstTab {
			*s.activeTab--
		} else {
			*s.activeTab = lastTab
		}
	}

	*s.focusIndex = f.getFirstFocusIndexForTab(*s.activeTab)
	*s.scrollOffset = 0
	return f.focusCurrentField()
}
