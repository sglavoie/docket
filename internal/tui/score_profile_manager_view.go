package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/scoring"
)

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

	if len(m.profiles) == 0 && !m.pickingPreset {
		lines = append(lines, MutedStyle.Render("No scoring profiles. Press 'a' to add a preset."))
	} else if len(m.profiles) > 0 {
		wideScreen := m.width >= 81
		var detailLines []string

		for i, p := range m.profiles {
			cursor := "  "
			style := dimStyle
			if i == m.cursor {
				style = selectedStyle
				if wideScreen {
					detailLines = m.buildDetailPane(&p, m.detailPaneWidth())
				}
			}

			check := "  "
			if p.ID == assignedID {
				check = checkStyle.Render(icons.Get("success") + " ")
			}

			summary := formulaStyle.Render(fmt.Sprintf("(%s)", p.Formula))
			dimCount := headerDimStyle.Render(fmt.Sprintf("[%d dims]", len(p.Dimensions)))
			line := fmt.Sprintf("%s%s%s %s %s", cursor, check, p.Name, summary, dimCount)
			lines = append(lines, style.Render(line))
		}

		if wideScreen && len(detailLines) > 0 {
			lines = m.renderWithDetailPane(lines, detailLines)
		}
	}

	if m.pickingPreset {
		lines = append(lines, "")
		lines = append(lines, titleStyle.Render("Select preset to add:"))
		lines = append(lines, m.renderPresetOptions(
			scoreProfileOverlayContentWidth(m.width),
			m.presetOptionLineBudget(len(lines)),
		)...)
	}

	if m.statusMessage != "" {
		lines = append(lines, "")
		lines = append(lines, MutedStyle.Render(m.statusMessage))
	}

	lines = append(lines, "")
	if m.pickingPreset {
		lines = append(lines, HelpStyle.Render("j/k: navigate  Enter: add  Esc: cancel"))
	} else {
		lines = append(lines, HelpStyle.Render("a: add preset  d: delete  Enter: assign  u: unassign  Esc: close"))
	}

	content := strings.Join(lines, "\n")
	boxStyle := OverlayBoxStyle
	if m.width > 0 {
		boxStyle = boxStyle.MaxWidth(m.width)
	}
	box := boxStyle.Render(content)

	return tea.NewView(lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box))
}

func scoreProfileOverlayContentWidth(width int) int {
	if width <= 0 {
		width = 80
	}
	chromeWidth := OverlayBoxStyle.GetHorizontalBorderSize() + OverlayBoxStyle.GetHorizontalPadding()
	return max(width-chromeWidth, 1)
}

func (m *ScoreProfileManagerModel) presetOptionLineBudget(usedLines int) int {
	if m.height <= 0 {
		return 0
	}

	const appStatusBarHeight = 2
	verticalChrome := OverlayBoxStyle.GetVerticalBorderSize() + OverlayBoxStyle.GetVerticalPadding()
	contentBudget := max(m.height-appStatusBarHeight-verticalChrome, 1)

	reservedAfterOptions := 2 // blank line + help line
	if m.statusMessage != "" {
		reservedAfterOptions += 2 // blank line + status line
	}

	return max(contentBudget-usedLines-reservedAfterOptions, 1)
}

func (m *ScoreProfileManagerModel) renderPresetOptions(maxWidth, maxLines int) []string {
	lines := m.renderPresetOptionsWithUsage(maxWidth, true)
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}
	return m.renderPresetOptionsWithUsage(maxWidth, false)
}

func (m *ScoreProfileManagerModel) renderPresetOptionsWithUsage(maxWidth int, includeUsageHints bool) []string {
	presets := scoring.PresetProfiles()
	lines := make([]string, 0, len(m.presetOptions))

	for _, name := range m.presetOptions {
		if factory, ok := presets[name]; ok {
			profile := factory()
			prefix := fmt.Sprintf("  %s", name)
			descWidth := max(maxWidth-lipgloss.Width(prefix), 1)
			descLines := wrapLine(" - "+profile.Description, descWidth)
			if len(descLines) == 0 {
				lines = append(lines, prefix)
			} else {
				lines = append(lines, prefix+MutedStyle.Render(descLines[0]))
				for _, line := range descLines[1:] {
					lines = append(lines, "    "+MutedStyle.Render(line))
				}
			}
			if includeUsageHints && profile.UsageHint != "" {
				for _, line := range wrapLine(profile.UsageHint, max(maxWidth-4, 1)) {
					lines = append(lines, "    "+MutedStyle.Render(line))
				}
			}
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s", name))
	}

	return lines
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
		case scoring.RoleNone:
			// no role suffix
		case scoring.RoleNumerator:
			roleStr = " [num]"
		case scoring.RoleDenominator:
			roleStr = " [den]"
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
		breakPoint := maxWidth
		for i := maxWidth; i > 0; i-- {
			if remaining[i-1] == ' ' {
				breakPoint = i - 1
				break
			}
		}
		lines = append(lines, remaining[:breakPoint])
		remaining = strings.TrimLeft(remaining[breakPoint:], " ")
	}
	if remaining != "" {
		lines = append(lines, remaining)
	}

	return lines
}

// renderWithDetailPane merges the profile list (left) with detail pane (right).
func (m *ScoreProfileManagerModel) renderWithDetailPane(leftLines, detailLines []string) []string {
	listStart := min(3, len(leftLines))
	header := leftLines[:listStart]
	listLines := leftLines[listStart:]

	leftWidth := min(40, m.width/2)
	rightWidth := m.detailPaneWidth()
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

		leftVis := lipgloss.Width(left)
		padding := max(leftWidth-leftVis, 0)
		rightVis := lipgloss.Width(right)
		rightPad := max(rightWidth-rightVis, 0)

		merged = append(merged, left+strings.Repeat(" ", padding)+separator+right+strings.Repeat(" ", rightPad))
	}

	return merged
}
