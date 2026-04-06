package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/scoring"
)

// ScoreProfileManagerModel manages scoring profiles and project assignment.
type ScoreProfileManagerModel struct {
	config           *config.Config
	configManager    *config.Manager
	profiles         []scoring.ScoringProfile
	cursor           int
	confirm          ConfirmDialog
	currentProjectID string
	statusMessage    string
	pendingPrefix    string

	// Inline preset picker sub-mode
	pickingPreset bool
	presetOptions []string
	presetCursor  int

	width  int
	height int
}

// NewScoreProfileManagerModel creates a new score profile manager view.
func NewScoreProfileManagerModel(cfg *config.Config, manager *config.Manager, currentProjectID string) *ScoreProfileManagerModel {
	profiles := make([]scoring.ScoringProfile, len(cfg.ScoringProfiles))
	copy(profiles, cfg.ScoringProfiles)
	return &ScoreProfileManagerModel{
		config:           cfg,
		configManager:    manager,
		profiles:         profiles,
		currentProjectID: currentProjectID,
	}
}

// setStatus sets the status message.
func (m *ScoreProfileManagerModel) setStatus(msg string) {
	m.statusMessage = msg
}

// SetSize implements View.
func (m *ScoreProfileManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ScoreProfileManagerModel) GetConfirmDialog() *ConfirmDialog { return &m.confirm }

func (m *ScoreProfileManagerModel) Init() tea.Cmd {
	return nil
}

func (m *ScoreProfileManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		// Handle confirm dialog first
		if m.confirm.IsActive() {
			confirmed, handled := m.confirm.HandleKey(msg.String())
			if handled {
				if confirmed {
					return m, m.executeDelete()
				}
				return m, nil
			}
		}

		// Handle preset picker sub-mode
		if m.pickingPreset {
			return m.handlePresetPicker(msg)
		}

		return m.handleKeypress(msg)
	}
	return m, nil
}

func (m *ScoreProfileManagerModel) HandledKeys() []string {
	return []string{
		"j", "down", "k", "up", "g", "G",
		"a", "d", "enter", "space", "u",
	}
}

func (m *ScoreProfileManagerModel) handleKeypress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys
	if m.pendingPrefix != "" {
		prefix := m.pendingPrefix
		m.pendingPrefix = ""
		if key != "esc" && prefix == "g" && key == "g" {
			m.cursor = 0
		}
		return m, nil
	}

	switch key {
	case "esc", "q":
		return m, func() tea.Msg {
			return CloseScoreProfileManagerMsg{}
		}

	case "j", "down":
		if m.cursor < len(m.profiles)-1 {
			m.cursor++
		}
		return m, nil

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "g":
		m.pendingPrefix = "g"
		return m, nil

	case "G":
		if len(m.profiles) > 0 {
			m.cursor = len(m.profiles) - 1
		}
		return m, nil

	case "a":
		return m, m.startAddPreset()

	case "d":
		if len(m.profiles) > 0 && m.cursor < len(m.profiles) {
			profile := m.profiles[m.cursor]
			m.confirm.Show("delete", fmt.Sprintf("Delete profile %q?", profile.Name))
		}
		return m, nil

	case "enter", " ":
		return m, m.assignProfile()

	case "u":
		return m, m.unassignProfile()
	}

	return m, nil
}

func (m *ScoreProfileManagerModel) handlePresetPicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.pickingPreset = false
		return m, nil

	case "j", "down":
		if m.presetCursor < len(m.presetOptions)-1 {
			m.presetCursor++
		}
		return m, nil

	case "k", "up":
		if m.presetCursor > 0 {
			m.presetCursor--
		}
		return m, nil

	case "enter":
		return m, m.addPreset(m.presetOptions[m.presetCursor])
	}

	return m, nil
}

// startAddPreset computes available presets and enters picker mode or adds directly.
func (m *ScoreProfileManagerModel) startAddPreset() tea.Cmd {
	existing := make(map[string]bool)
	for _, p := range m.profiles {
		existing[p.ID] = true
	}

	var available []string
	for _, name := range scoring.PresetNames() {
		if !existing[name] {
			available = append(available, name)
		}
	}

	if len(available) == 0 {
		m.setStatus("All preset profiles already added")
		return defaultStatusTimeout()
	}

	if len(available) == 1 {
		return m.addPreset(available[0])
	}

	// Multiple available: enter picker sub-mode
	m.presetOptions = available
	m.presetCursor = 0
	m.pickingPreset = true
	return nil
}

// addPreset adds a preset profile by name.
func (m *ScoreProfileManagerModel) addPreset(name string) tea.Cmd {
	m.pickingPreset = false
	presets := scoring.PresetProfiles()
	factory, ok := presets[name]
	if !ok {
		return nil
	}

	profile := factory()
	if err := m.config.AddScoringProfile(profile); err != nil {
		m.setStatus(err.Error())
		return defaultStatusTimeout()
	}

	if err := m.configManager.Save(m.config); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return defaultStatusTimeout()
	}

	// Refresh snapshot
	m.profiles = make([]scoring.ScoringProfile, len(m.config.ScoringProfiles))
	copy(m.profiles, m.config.ScoringProfiles)

	// Move cursor to the new profile
	for i, p := range m.profiles {
		if p.ID == profile.ID {
			m.cursor = i
			break
		}
	}

	m.setStatus(fmt.Sprintf("Added %s profile", profile.Name))
	return tea.Batch(
		defaultStatusTimeout(),
		func() tea.Msg { return ScoreProfileChangedMsg{} },
	)
}

// executeDelete removes the profile at cursor.
func (m *ScoreProfileManagerModel) executeDelete() tea.Cmd {
	if m.cursor >= len(m.profiles) {
		return nil
	}

	profile := m.profiles[m.cursor]

	// Clean up project assignments that reference this profile
	if m.config.ProjectScoringProfile != nil {
		for pid, profID := range m.config.ProjectScoringProfile {
			if profID == profile.ID {
				delete(m.config.ProjectScoringProfile, pid)
			}
		}
	}

	m.config.RemoveScoringProfile(profile.ID)
	if err := m.configManager.Save(m.config); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return defaultStatusTimeout()
	}

	// Refresh snapshot
	m.profiles = make([]scoring.ScoringProfile, len(m.config.ScoringProfiles))
	copy(m.profiles, m.config.ScoringProfiles)

	if m.cursor >= len(m.profiles) && m.cursor > 0 {
		m.cursor--
	}

	m.setStatus(fmt.Sprintf("Deleted profile %q", profile.Name))
	return tea.Batch(
		defaultStatusTimeout(),
		func() tea.Msg { return ScoreProfileChangedMsg{} },
	)
}

// assignProfile assigns the selected profile to the current project.
func (m *ScoreProfileManagerModel) assignProfile() tea.Cmd {
	if len(m.profiles) == 0 || m.cursor >= len(m.profiles) || m.currentProjectID == "" {
		return nil
	}

	profile := m.profiles[m.cursor]
	m.config.SetProjectScoringProfile(m.currentProjectID, profile.ID)
	if err := m.configManager.Save(m.config); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return defaultStatusTimeout()
	}

	m.setStatus(fmt.Sprintf("Assigned %s to project", profile.Name))
	return tea.Batch(
		defaultStatusTimeout(),
		func() tea.Msg { return ScoreProfileChangedMsg{} },
	)
}

// unassignProfile removes the scoring profile from the current project.
func (m *ScoreProfileManagerModel) unassignProfile() tea.Cmd {
	if m.currentProjectID == "" {
		return nil
	}

	m.config.UnsetProjectScoringProfile(m.currentProjectID)
	if err := m.configManager.Save(m.config); err != nil {
		m.setStatus("Failed to save: " + err.Error())
		return defaultStatusTimeout()
	}

	m.setStatus("Unassigned scoring profile from project")
	return tea.Batch(
		defaultStatusTimeout(),
		func() tea.Msg { return ScoreProfileChangedMsg{} },
	)
}

// assignedProfileID returns the profile ID assigned to the current project, or "".
func (m *ScoreProfileManagerModel) assignedProfileID() string {
	if m.config.ProjectScoringProfile == nil || m.currentProjectID == "" {
		return ""
	}
	return m.config.ProjectScoringProfile[m.currentProjectID]
}

func (m *ScoreProfileManagerModel) View() tea.View {
	titleStyle := OverlayTitleStyle
	subtitleStyle := MutedStyle
	selectedStyle := OverlaySelectedStyle
	dimStyle := NormalStyle
	checkStyle := CheckmarkStyle
	formulaStyle := MutedStyle
	headerDimStyle := MutedStyle

	assignedID := m.assignedProfileID()
	assignedName := "none"
	if assignedID != "" {
		for _, p := range m.profiles {
			if p.ID == assignedID {
				assignedName = p.Name
				break
			}
		}
	}

	var lines []string
	lines = append(lines, titleStyle.Render("Scoring Profiles"))
	lines = append(lines, subtitleStyle.Render(fmt.Sprintf("Assigned to project: %s", assignedName)))
	lines = append(lines, "")

	if len(m.profiles) == 0 {
		lines = append(lines, MutedStyle.Render("No scoring profiles. Press 'a' to add a preset."))
	} else {
		wideScreen := m.width >= 81
		var detailLines []string

		for i, p := range m.profiles {
			cursor := "  "
			style := dimStyle
			if i == m.cursor {
				cursor = "  "
				style = selectedStyle

				// Build detail pane content for wide screens
				if wideScreen {
					detailLines = m.buildDetailPane(&p, m.detailPaneWidth())
				}
			}

			// Check indicator for assigned profile
			check := "  "
			if p.ID == assignedID {
				check = checkStyle.Render("✓ ")
			}

			summary := formulaStyle.Render(fmt.Sprintf("(%s)", p.Formula))
			dimCount := headerDimStyle.Render(fmt.Sprintf("[%d dims]", len(p.Dimensions)))

			line := fmt.Sprintf("%s%s%s %s %s", cursor, check, p.Name, summary, dimCount)
			lines = append(lines, style.Render(line))
		}

		// Render with side-by-side detail pane on wide screens
		if wideScreen && len(detailLines) > 0 {
			lines = m.renderWithDetailPane(lines, detailLines)
		}
	}

	// Preset picker sub-mode
	if m.pickingPreset {
		lines = append(lines, "")
		lines = append(lines, titleStyle.Render("Select preset to add:"))
		for i, name := range m.presetOptions {
			cursor := "  "
			if i == m.presetCursor {
				cursor = "  "
			}
			presets := scoring.PresetProfiles()
			desc := ""
			if factory, ok := presets[name]; ok {
				p := factory()
				desc = " - " + p.Description
				if p.UsageHint != "" {
					desc += "\n    " + p.UsageHint
				}
			}
			lines = append(lines, fmt.Sprintf("%s%s%s", cursor, name, MutedStyle.Render(desc)))
		}
	}

	// Status message
	if m.statusMessage != "" {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render(m.statusMessage))
	}

	lines = append(lines, "")
	if m.pickingPreset {
		lines = append(lines, HelpStyle.Render("j/k: navigate  Enter: add  Esc: cancel"))
	} else {
		lines = append(lines, HelpStyle.Render("a: add preset  d: delete  Enter: assign  u: unassign  q: close"))
	}

	content := strings.Join(lines, "\n")

	box := OverlayBoxStyle.Render(content)

	return tea.NewView(lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box))
}

// detailPaneWidth computes the fixed width for the right detail pane.
func (m *ScoreProfileManagerModel) detailPaneWidth() int {
	leftWidth := min(40, m.width/2)
	separatorWidth := 3
	totalContentWidth := max(m.width*70/100, leftWidth+separatorWidth+10)
	rightWidth := max(totalContentWidth-leftWidth-separatorWidth, 10)
	return rightWidth
}

// buildDetailPane returns lines describing the selected profile's dimensions.
func (m *ScoreProfileManagerModel) buildDetailPane(p *scoring.ScoringProfile, maxWidth int) []string {
	headerStyle := OverlayTitleStyle
	dimStyle := OverlayDetailStyle
	mutedStyle := MutedStyle

	var lines []string
	lines = append(lines, headerStyle.Render(p.Name))
	if p.Description != "" {
		for _, wl := range wrapLine(p.Description, maxWidth) {
			lines = append(lines, mutedStyle.Render(wl))
		}
	}
	if p.UsageHint != "" {
		for _, wl := range wrapLine(p.UsageHint, maxWidth) {
			lines = append(lines, mutedStyle.Render(wl))
		}
	}
	lines = append(lines, mutedStyle.Render(fmt.Sprintf("Formula: %s", p.Formula)))
	lines = append(lines, "")

	for _, d := range p.Dimensions {
		roleStr := ""
		switch d.Role {
		case scoring.RoleNumerator:
			roleStr = " [num]"
		case scoring.RoleDenominator:
			roleStr = " [den]"
		default:
		}

		sourceStr := string(d.Source)
		rangeStr := fmt.Sprintf("%d-%d", d.Min, d.Max)

		line := fmt.Sprintf("  %s%s: %s (%s)", d.Name, roleStr, rangeStr, sourceStr)
		lines = append(lines, dimStyle.Render(line))
	}

	return lines
}

// wrapLine splits text into lines that fit within maxWidth using word boundaries.
func wrapLine(text string, maxWidth int) []string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return []string{text}
	}
	var lines []string
	remaining := text
	for len(remaining) > maxWidth {
		bp := maxWidth
		for i := maxWidth; i > 0; i-- {
			if remaining[i-1] == ' ' {
				bp = i - 1
				break
			}
		}
		lines = append(lines, remaining[:bp])
		remaining = strings.TrimLeft(remaining[bp:], " ")
	}
	if remaining != "" {
		lines = append(lines, remaining)
	}
	return lines
}

// renderWithDetailPane merges the profile list (left) with detail pane (right).
func (m *ScoreProfileManagerModel) renderWithDetailPane(leftLines, detailLines []string) []string {
	// Find the separator point: after the blank line following subtitle
	listStart := min(
		// title + subtitle + blank line
		3, len(leftLines))

	// Split header vs list
	header := leftLines[:listStart]
	listLines := leftLines[listStart:]

	// Determine pane widths
	leftWidth := min(40, m.width/2)

	rightWidth := m.detailPaneWidth()

	// Build merged lines
	maxLines := max(len(detailLines), len(listLines))

	var merged []string
	merged = append(merged, header...)

	separator := MutedStyle.Render(" │ ")
	truncStyle := lipgloss.NewStyle().MaxWidth(rightWidth)

	for i := range maxLines {
		left := ""
		if i < len(listLines) {
			left = listLines[i]
		}
		right := ""
		if i < len(detailLines) {
			right = truncStyle.Render(detailLines[i])
		}

		// Pad left side (use visual width)
		leftVis := lipgloss.Width(left)
		padding := max(leftWidth-leftVis, 0)

		// Pad right side to fixed width
		rightVis := lipgloss.Width(right)
		rightPad := max(rightWidth-rightVis, 0)

		merged = append(merged, left+strings.Repeat(" ", padding)+separator+right+strings.Repeat(" ", rightPad))
	}

	return merged
}

// ShouldCaptureEsc returns true when confirm or preset picker is active.
func (m *ScoreProfileManagerModel) ShouldCaptureEsc() bool {
	return m.confirm.IsActive() || m.pickingPreset
}

// HasActiveSearch returns false.
func (m *ScoreProfileManagerModel) HasActiveSearch() bool {
	return false
}

// GetHelpItems returns help items for this view.
func (m *ScoreProfileManagerModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewScoreProfileManager)
}

func (m *ScoreProfileManagerModel) StatusBarLabel() string { return "Score Profiles" }
