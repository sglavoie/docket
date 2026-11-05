package help

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

const (
	commandPaletteTitle = "Command Palette"
	// commandPaletteMaxRows caps the result window. The window is padded to its
	// height, so the box holds still while a query narrows the list.
	commandPaletteMaxRows = 12
	// commandPaletteChromeRows is the halo, the frame, the title, the input, and
	// the host's title row, which the shell keeps above every transient popup.
	commandPaletteChromeRows = 7
)

// commandPaletteInnerWidth matches the lens picker and the conflict browser, so
// every shell modal sits in the same frame at every terminal size.
func commandPaletteInnerWidth(width int) int {
	return min(max(width*4/5, 46), max(width-6, 24))
}

func (p *CommandPaletteModel) Render(width int) string {
	return p.RenderWithin(width, 0)
}

// RenderWithin spends whatever the frame leaves after the chrome, the selected
// row's context line, and the guidance on result rows, so the minimum terminal
// drops results rather than the input, the reason, or the exits.
func (p *CommandPaletteModel) RenderWithin(width, height int) string {
	if p == nil {
		return ""
	}
	innerWidth := commandPaletteInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)
	footer := chrome.RenderGuidanceText(p.guidanceSpec(), contentWidth)
	context := p.contextLines(contentWidth)
	rows := commandPaletteMaxRows
	if height > 0 {
		rows = min(rows, max(height-commandPaletteChromeRows-lipgloss.Height(footer)-len(context), 1))
	}
	body := []string{p.input.View(p.query, true, contentWidth)}
	body = append(body, p.renderResults(contentWidth, rows)...)
	body = append(body, context...)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(commandPaletteTitle)
	return chrome.RenderOverlayBox(strings.Join(body, "\n"), innerWidth, title, footer,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary))
}

// renderResults draws exactly rows lines: the window of results holding the
// cursor's whole row, padded below.
func (p *CommandPaletteModel) renderResults(width, rows int) []string {
	p.pageRows = rows
	lines := make([]string, 0, rows)
	if len(p.results) == 0 {
		lines = append(lines, theme.MutedStyle.Render(textfit.TruncateIntentional("No matching actions.", width, "…")))
	} else {
		keyWidth := p.keyColumnWidth(width)
		p.top = min(p.top, p.cursor)
		for p.top < p.cursor && p.linesBetween(p.top, p.cursor, width, keyWidth) > rows {
			p.top++
		}
		for index := p.top; index < len(p.results) && len(lines) < rows; index++ {
			lines = append(lines, p.renderResult(index, width, keyWidth)...)
		}
	}
	if len(lines) > rows {
		lines = lines[:rows]
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return lines
}

func (p *CommandPaletteModel) linesBetween(from, to, width, keyWidth int) int {
	total := 0
	for index := from; index <= to; index++ {
		total += len(p.renderResult(index, width, keyWidth))
	}
	return total
}

// keyColumnWidth is the widest key any action spells, capped at a third of the
// row so a long chord list wraps inside its column rather than taking the label's.
func (p *CommandPaletteModel) keyColumnWidth(width int) int {
	widest := 1
	for _, item := range p.items {
		widest = max(widest, lipgloss.Width(item.Key))
	}
	return max(min(widest, width/3), 1)
}

// renderResult is one action: its label and section, wrapped, and its key in a
// right-hand column. Nothing is truncated, so a long label costs a row rather
// than its tail.
func (p *CommandPaletteModel) renderResult(index, width, keyWidth int) []string {
	item := p.results[index]
	textWidth := max(width-2-1-keyWidth, 1)
	disabled := item.DisabledReason != ""
	label := item.Description
	if !disabled {
		label = textfit.HighlightText(item.Description, item.descPositions, theme.NormalStyle)
	}
	section := "  " + helpSectionForItem(item.HelpItem)
	if len(item.matchedAliases) > 0 {
		section += " · " + strings.Join(item.matchedAliases, ", ")
	}
	text := textfit.WrapStyled(label+theme.MutedStyle.Render(section), textWidth)
	keys := textfit.WrapText(item.Key, keyWidth)
	if len(keys) == 1 && !disabled {
		keys[0] = textfit.HighlightText(item.Key, item.keyPositions, theme.TagInlineStyle)
	}
	lines := make([]string, 0, max(len(text), len(keys)))
	for row := 0; row < max(len(text), len(keys)); row++ {
		marker := "  "
		if row == 0 && index == p.cursor {
			marker = "› "
		}
		left, right := "", ""
		if row < len(text) {
			left = text[row]
		}
		if row < len(keys) {
			right = keys[row]
		}
		line := marker + left + strings.Repeat(" ", max(textWidth-lipgloss.Width(left), 0)) + " " +
			strings.Repeat(" ", max(keyWidth-lipgloss.Width(right), 0)) + right
		if disabled {
			line = theme.MutedStyle.Render(line)
		}
		if index == p.cursor {
			line = chrome.SelectedLine(line, width)
		}
		lines = append(lines, line)
	}
	return lines
}

// contextLines say what Enter would act on, or why it would not: the frozen
// target for an available row, the availability reason for one that is not.
func (p *CommandPaletteModel) contextLines(width int) []string {
	item, ok := p.Selected()
	if !ok {
		return nil
	}
	if item.DisabledReason != "" {
		lines := textfit.WrapText(item.DisabledReason, width)
		for i := range lines {
			lines[i] = theme.MutedStyle.Render(lines[i])
		}
		return lines
	}
	if item.Target == "" {
		return nil
	}
	return []string{theme.MutedStyle.Render(textfit.TruncateIntentional("Target: "+item.Target, width, "…"))}
}

// guidanceSpec is the palette's own footer. The modal owns every key, so the
// shell footer beneath it is suppressed and these exits are the frame's only
// ones; Enter dims over a row it would refuse.
func (p *CommandPaletteModel) guidanceSpec() chrome.GuidanceSpec {
	spec := chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints("Enter run", "Esc close", "↑/↓ choose"), Rows: 2}
	if item, ok := p.Selected(); !ok || item.DisabledReason != "" {
		spec.Hints[0].Dimmed = true
	}
	noun := "actions"
	if len(p.results) == 1 {
		noun = "action"
	}
	spec.Supplemental = fmt.Sprintf("%d %s", len(p.results), noun)
	return spec
}
