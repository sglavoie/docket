package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// handleAxisManagerKey processes key events when the axis manager popup is open.
func (m *KanbanModel) handleAxisManagerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	modes := AllKanbanModes()

	switch msg.String() {
	case "j", "down":
		m.axisCursor = (m.axisCursor + 1) % len(modes)
	case "k", "up":
		m.axisCursor = (m.axisCursor - 1 + len(modes)) % len(modes)
	case "space":
		m.toggleAxisMode(modes[m.axisCursor])
	case "@", "esc":
		m.showAxisManager = false
	}
	return m, nil
}

// toggleAxisMode enables or disables the given mode in enabledModes, with a
// guard that prevents disabling the last enabled mode.
func (m *KanbanModel) toggleAxisMode(mode KanbanMode) {
	enabled := m.enabledModeList()

	// Check if mode is currently enabled
	isEnabled := slices.Contains(enabled, mode)

	if isEnabled {
		// Guard: don't disable last enabled mode
		if len(enabled) <= 1 {
			m.statusMsg = "At least one axis mode must remain enabled"
			return
		}
		// Remove mode, preserving order
		var updated []KanbanMode
		for _, em := range enabled {
			if em != mode {
				updated = append(updated, em)
			}
		}
		m.enabledModes = updated
	} else {
		// Add mode — insert in canonical order
		enabledSet := make(map[KanbanMode]bool, len(enabled)+1)
		for _, em := range enabled {
			enabledSet[em] = true
		}
		enabledSet[mode] = true
		var updated []KanbanMode
		for _, cm := range AllKanbanModes() {
			if enabledSet[cm] {
				updated = append(updated, cm)
			}
		}
		m.enabledModes = updated
	}

	// Persist enabled modes to the portable Settings domain.
	enabledIDs := make([]string, 0, len(m.enabledModes))
	for _, em := range m.enabledModes {
		enabledIDs = append(enabledIDs, em.ID())
	}
	_ = commitSettingsToRuntime(m.runtime, func(settings *config.Settings) {
		settings.KanbanSettings.EnabledModes = enabledIDs
	})

	// If current mode is no longer enabled, switch to first enabled mode
	currentEnabled := slices.Contains(m.enabledModeList(), m.mode)
	if !currentEnabled && len(m.enabledModeList()) > 0 {
		m.mode = m.enabledModeList()[0]
		m.rebuildCols()
		m.clampCursor()
	}
}

// handleModePickerKey processes key events when the mode picker popup is open.
func (m *KanbanModel) handleModePickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	modes := AllKanbanModes()
	key := msg.String()
	switch key {
	case "esc", "`":
		m.showModePicker = false
	default:
		for i, mode := range modes {
			if key == fmt.Sprintf("%d", i+1) {
				m.showModePicker = false
				m.mode = mode
				m.rebuildCols()
				m.clampCursor()
				break
			}
		}
	}
	return m, nil
}

// buildModePickerHints builds the PrefixHints for the mode picker popup.
func (m *KanbanModel) buildModePickerHints() PrefixHints {
	modes := AllKanbanModes()
	hints := make([]PrefixHint, 0, len(modes))
	for i, mode := range modes {
		icon := KanbanModeIcons[mode]
		name := KanbanModeNames[mode]
		hints = append(hints, PrefixHint{
			Key:          fmt.Sprintf("%d", i+1),
			Description:  name,
			RenderedIcon: icon,
			Active:       mode == m.mode,
		})
	}
	return PrefixHints{"`": hints}
}

// cycleAxisMode advances to the next enabled mode (`).
func (m *KanbanModel) cycleAxisMode() {
	m.cycleAxisModeBy(1)
}

// cycleAxisModeBack moves to the previous enabled mode (~).
func (m *KanbanModel) cycleAxisModeBack() {
	m.cycleAxisModeBy(-1)
}

// cycleAxisModeBy cycles the mode by delta (+1 forward, -1 backward).
func (m *KanbanModel) cycleAxisModeBy(delta int) {
	enabled := m.enabledModeList()
	if len(enabled) <= 1 {
		return
	}
	cur := -1
	for i, em := range enabled {
		if em == m.mode {
			cur = i
			break
		}
	}
	if cur < 0 {
		cur = 0
	}
	m.mode = enabled[(cur+delta+len(enabled))%len(enabled)]
	m.rebuildCols()
	m.clampCursor()
}

// renderAxisManagerPopup renders the axis mode manager popup as a string.
func (m *KanbanModel) renderAxisManagerPopup() string {
	modes := AllKanbanModes()
	enabled := m.enabledModeList()
	enabledSet := make(map[KanbanMode]bool, len(enabled))
	for _, em := range enabled {
		enabledSet[em] = true
	}

	title := TitleStyle.Render("Kanban Axis Modes")
	footerText := "space: toggle  j/k: move  esc: close"

	const popupW = 40
	innerW := popupW - 4 // border(2) + padding(2)

	var lines []string
	for i, mode := range modes {
		var checkbox string
		if enabledSet[mode] {
			checkbox = SelectionCheckStyle.Render("[x]")
		} else {
			checkbox = MutedStyle.Render("[ ]")
		}
		icon := KanbanModeIcons[mode]
		name := KanbanModeNames[mode]
		line := fmt.Sprintf(" %s %s %s", checkbox, icon, name)
		line = truncateAndPad(line, innerW)
		if i == m.axisCursor {
			line = SelectedLine(line, innerW)
		}
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	footer := MutedStyle.Render(footerText)
	return renderTitledBox(content, popupW, title, footer, InfoStatusStyle)
}
