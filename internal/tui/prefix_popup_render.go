package tui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// groupOrder defines the canonical display order for pipeline groups.
var groupOrder = []string{"Capture", "Organize", "Plan", "Review", "Support"}

// groupRank maps group name → sort rank for pipeline groups.
var groupRank = func() map[string]int {
	m := make(map[string]int, len(groupOrder))
	for i, g := range groupOrder {
		m[g] = i
	}
	return m
}()

// groupAccentColor maps group name → pipeline accent color.
var groupAccentColor = map[string]color.Color{
	"Capture":  AccentCapture,
	"Organize": AccentOrganize,
	"Plan":     AccentPlan,
	"Review":   AccentReview,
	"Support":  AccentSupport,
}

// groupBgColor maps group name → subtle dark background tint.
var groupBgColor = map[string]color.Color{
	"Capture":  lipgloss.Color("#3a2222"),
	"Organize": lipgloss.Color("#1e2a3a"),
	"Plan":     lipgloss.Color("#1e3322"),
	"Review":   lipgloss.Color("#332e1a"),
	"Support":  lipgloss.Color("#2c2c2c"),
}

// groupLightColor maps group name → lighter/pastel variant for detail text.
var groupLightColor = map[string]color.Color{
	"Capture":  lipgloss.Color("#e0a090"),
	"Organize": lipgloss.Color("#90c0e0"),
	"Plan":     lipgloss.Color("#80d0a0"),
	"Review":   lipgloss.Color("#e0d090"),
	"Support":  lipgloss.Color("#a0a0a0"),
}

// hasGroups returns true if any hint has a non-empty Group field.
func hasGroups(hints []PrefixHint) bool {
	for _, h := range hints {
		if h.Group != "" {
			return true
		}
	}
	return false
}

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
	titleCatRestStyle := NormalStyle

	var categoryTitle string
	if strings.EqualFold(prefix, string(category[0])) {
		categoryTitle = titleCatFirstStyle.Render(string(category[0])) + titleCatRestStyle.Render(category[1:])
	} else {
		categoryTitle = titleCatRestStyle.Render(category)
	}
	title := titleKeyStyle.Render(prefix) + borderStyle.Render(" → ") + categoryTitle

	maxAvailableHeight := max(
		min(height-4, 30),
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
		if hasGroups(keyHints) {
			contentLines, contentWidth := renderGroupedPrefixHintLines(keyHints)
			boxInnerWidth := min(max(contentWidth+4, minWidth), maxWidth)
			return renderPrefixBox(title, contentLines, boxInnerWidth, borderStyle)
		}
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
		rendered, lineWidth := renderAlignedPrefixHintLine(hint, maxKeyWidth, maxDescWidth, maxIconWidth, iconColWidth, hasDetail, hasIcon, "")
		contentLines = append(contentLines, rendered)
		if lineWidth > contentWidth {
			contentWidth = lineWidth
		}
	}

	return contentLines, contentWidth
}

// renderGroupedPrefixHintLines renders hints sorted into pipeline groups with
// group names displayed as left-side labels on the first item of each group.
func renderGroupedPrefixHintLines(hints []PrefixHint) ([]string, int) {
	maxKeyWidth := 0
	maxDescWidth := 0
	maxIconWidth := 0
	maxDetailWidth := 0
	hasDetail := false
	hasIcon := false

	for _, hint := range hints {
		if len(hint.Key) > maxKeyWidth {
			maxKeyWidth = len(hint.Key)
		}
		if len(hint.Description) > maxDescWidth {
			maxDescWidth = len(hint.Description)
		}
		if hint.Detail != "" {
			hasDetail = true
			if len(hint.Detail) > maxDetailWidth {
				maxDetailWidth = len(hint.Detail)
			}
		}
		if hint.RenderedIcon != "" {
			hasIcon = true
			if w := ansi.StringWidth(hint.RenderedIcon); w > maxIconWidth {
				maxIconWidth = w
			}
		}
	}

	iconColWidth := 0
	if hasIcon {
		iconColWidth = maxIconWidth + 1
	}

	// Compute max group name width for left-label column.
	maxGroupWidth := 0
	for _, g := range groupOrder {
		if len(g) > maxGroupWidth {
			maxGroupWidth = len(g)
		}
	}
	groupColWidth := maxGroupWidth + 2 // name + "  " gap

	// Compute content line width from column widths.
	contentWidth := groupColWidth + maxKeyWidth + 2 + iconColWidth + maxDescWidth
	if hasDetail {
		contentWidth += 2 + maxDetailWidth
	}

	// Stable-sort by group rank, then lowercase-first within group.
	// Empty-group entries sort before all named groups.
	sorted := make([]PrefixHint, len(hints))
	copy(sorted, hints)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri, rj := -1, -1
		if sorted[i].Group != "" {
			ri = groupRank[sorted[i].Group]
		}
		if sorted[j].Group != "" {
			rj = groupRank[sorted[j].Group]
		}
		if ri != rj {
			return ri < rj
		}
		a, b := sorted[i].Key, sorted[j].Key
		aLower := len(a) > 0 && a[0] >= 'a' && a[0] <= 'z'
		bLower := len(b) > 0 && b[0] >= 'a' && b[0] <= 'z'
		aUpper := len(a) > 0 && a[0] >= 'A' && a[0] <= 'Z'
		bUpper := len(b) > 0 && b[0] >= 'A' && b[0] <= 'Z'
		aRank := 1
		if aLower {
			aRank = 0
		} else if aUpper {
			aRank = 2
		}
		bRank := 1
		if bLower {
			bRank = 0
		} else if bUpper {
			bRank = 2
		}
		if aRank != bRank {
			return aRank < bRank
		}
		return a < b
	})

	var contentLines []string
	currentGroup := "\x00" // sentinel: no group yet
	blankGroupCol := strings.Repeat(" ", groupColWidth)

	for _, hint := range sorted {
		rendered, _ := renderAlignedPrefixHintLine(hint, maxKeyWidth, maxDescWidth, maxIconWidth, iconColWidth, hasDetail, hasIcon, hint.Group)

		if hint.Group != currentGroup {
			currentGroup = hint.Group
			if currentGroup != "" {
				fgStyle := lipgloss.NewStyle()
				if accent, ok := groupAccentColor[currentGroup]; ok {
					fgStyle = fgStyle.Foreground(accent)
				}
				if bg, ok := groupBgColor[currentGroup]; ok {
					fgStyle = fgStyle.Background(bg)
				}
				label := fgStyle.Render(fmt.Sprintf(" %-*s ", maxGroupWidth, currentGroup))
				contentLines = append(contentLines, label+rendered)
			} else {
				contentLines = append(contentLines, blankGroupCol+rendered)
			}
		} else {
			contentLines = append(contentLines, blankGroupCol+rendered)
		}
	}

	return contentLines, contentWidth
}

func renderAlignedPrefixHintLine(hint PrefixHint, maxKeyWidth, maxDescWidth, maxIconWidth, iconColWidth int, hasDetail, hasIcon bool, group string) (string, int) {
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
	} else if group != "" {
		if accent, ok := groupAccentColor[group]; ok {
			descStyle = lipgloss.NewStyle().Foreground(accent)
		}
		if light, ok := groupLightColor[group]; ok {
			detailStyle = lipgloss.NewStyle().Foreground(light)
		}
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
