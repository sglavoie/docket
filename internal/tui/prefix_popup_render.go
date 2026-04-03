package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// RenderPrefixPopup renders a which-key style popup as a bordered overlay box.
// Returns empty string if prefix is empty or not found in hints.
func RenderPrefixPopup(prefix string, hints PrefixHints, width, height int) string {
	if prefix == "" {
		return ""
	}

	keyHints, ok := hints[prefix]
	if !ok || len(keyHints) == 0 {
		return ""
	}

	category := prefixCategoryNames[prefix]
	if category == "" {
		category = prefix
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	titleKeyStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	titleCatFirstStyle := SuccessBoldStyle
	titleCatRestStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

	var categoryTitle string
	if strings.EqualFold(prefix, string(category[0])) {
		categoryTitle = titleCatFirstStyle.Render(string(category[0])) + titleCatRestStyle.Render(category[1:])
	} else {
		categoryTitle = titleCatRestStyle.Render(category)
	}
	title := titleKeyStyle.Render(prefix) + borderStyle.Render(" → ") + categoryTitle

	maxAvailableHeight := max(
		min(height-4, 20),
		3,
	)
	onePerLine := len(keyHints) <= maxAvailableHeight

	minWidth := 25
	maxWidth := width / 2
	if maxWidth < minWidth {
		maxWidth = width - 4
	}
	if maxWidth < minWidth {
		maxWidth = minWidth
	}

	if onePerLine {
		contentLines, contentWidth := renderAlignedPrefixHintLines(keyHints)
		boxInnerWidth := min(max(contentWidth+4, minWidth), maxWidth)
		return renderPrefixBox(title, contentLines, boxInnerWidth, borderStyle)
	}

	boxInnerWidth := maxWidth
	contentLines := renderCompactPrefixHintLines(keyHints, boxInnerWidth-4)
	return renderPrefixBox(title, contentLines, boxInnerWidth, borderStyle)
}

func renderAlignedPrefixHintLines(keyHints []PrefixHint) ([]string, int) {
	maxKeyWidth := 0
	maxDescWidth := 0
	maxIconWidth := 0
	hasDetail := false
	hasIcon := false

	for _, hint := range keyHints {
		if len(hint.Key) > maxKeyWidth {
			maxKeyWidth = len(hint.Key)
		}
		if len(hint.Description) > maxDescWidth {
			maxDescWidth = len(hint.Description)
		}
		if hint.Detail != "" {
			hasDetail = true
		}
		if hint.RenderedIcon != "" {
			hasIcon = true
			if width := ansi.StringWidth(hint.RenderedIcon); width > maxIconWidth {
				maxIconWidth = width
			}
		}
	}

	iconColWidth := 0
	if hasIcon {
		iconColWidth = maxIconWidth + 1
	}

	contentWidth := 0
	contentLines := make([]string, 0, len(keyHints))
	for _, hint := range keyHints {
		rendered, lineWidth := renderAlignedPrefixHintLine(hint, maxKeyWidth, maxDescWidth, maxIconWidth, iconColWidth, hasDetail, hasIcon)
		contentLines = append(contentLines, rendered)
		if lineWidth > contentWidth {
			contentWidth = lineWidth
		}
	}

	return contentLines, contentWidth
}

func renderAlignedPrefixHintLine(hint PrefixHint, maxKeyWidth, maxDescWidth, maxIconWidth, iconColWidth int, hasDetail, hasIcon bool) (string, int) {
	keyStyle := PrefixKeyStyle
	descStyle := PrefixDescStyle
	detailStyle := MutedStyle
	if hint.Disabled {
		keyStyle = MutedStyle
		descStyle = MutedStyle
		detailStyle = MutedStyle
	} else if hint.Active {
		descStyle = WarningIconStyle
		detailStyle = WarningIconStyle
	}

	iconPart := ""
	iconPlain := ""
	if hasIcon {
		if hint.RenderedIcon != "" {
			iconPart = hint.RenderedIcon + " "
			iconPlain = fmt.Sprintf("%-*s ", maxIconWidth, strings.Repeat("X", ansi.StringWidth(hint.RenderedIcon)))
		} else {
			iconPart = strings.Repeat(" ", iconColWidth)
			iconPlain = strings.Repeat(" ", iconColWidth)
		}
	}

	if hasDetail {
		plain := fmt.Sprintf("%-*s  %s%-*s  %s", maxKeyWidth, hint.Key, iconPlain, maxDescWidth, hint.Description, hint.Detail)
		rendered := keyStyle.Render(fmt.Sprintf("%-*s", maxKeyWidth, hint.Key)) + "  " +
			iconPart +
			descStyle.Render(fmt.Sprintf("%-*s", maxDescWidth, hint.Description)) + "  " +
			detailStyle.Render(hint.Detail)
		return rendered, len(plain)
	}

	plain := fmt.Sprintf("%-*s  %s%s", maxKeyWidth, hint.Key, iconPlain, hint.Description)
	rendered := keyStyle.Render(fmt.Sprintf("%-*s", maxKeyWidth, hint.Key)) + "  " + iconPart + descStyle.Render(hint.Description)
	return rendered, len(plain)
}

func renderCompactPrefixHintLines(keyHints []PrefixHint, availLineWidth int) []string {
	parts := make([]string, 0, len(keyHints))
	for _, hint := range keyHints {
		parts = append(parts, PrefixDescStyle.Render(hint.Description)+PrefixKeyStyle.Render("("+hint.Key+")"))
	}

	var contentLines []string
	var currentLine string
	currentWidth := 0
	for i, part := range parts {
		partWidth := lipgloss.Width(part)
		sep := ""
		sepWidth := 0
		if i > 0 {
			sep = " "
			sepWidth = 1
		}

		if currentWidth+sepWidth+partWidth > availLineWidth && currentLine != "" {
			contentLines = append(contentLines, currentLine)
			currentLine = part
			currentWidth = partWidth
			continue
		}

		if sep != "" {
			currentLine += sep
			currentWidth += sepWidth
		}
		currentLine += part
		currentWidth += partWidth
	}

	if currentLine != "" {
		contentLines = append(contentLines, currentLine)
	}

	return contentLines
}

// renderPrefixBox renders the bordered box with title and bottom "Esc" hint.
func renderPrefixBox(title string, contentLines []string, innerWidth int, borderStyle lipgloss.Style) string {
	content := strings.Join(contentLines, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content)
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}

	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(MutedStyle.Render("Esc to cancel"), actualWidth, borderStyle)

	return strings.Join(lines, "\n")
}
