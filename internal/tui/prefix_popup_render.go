package tui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// groupOrder defines the canonical display order for pipeline groups.
var groupOrder = []string{"Capture", "Organize", "Plan", "Work", "Review", "Support"}

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
	"Work":     AccentWork,
	"Review":   AccentReview,
	"Support":  AccentSupport,
}

// groupBgColor maps group name → subtle background tint.
var groupBgColor = map[string]color.Color{
	"Capture":  lipgloss.Color("#2a1f2a"),
	"Organize": lipgloss.Color("#1e2535"),
	"Plan":     lipgloss.Color("#1e2a22"),
	"Work":     lipgloss.Color("#2a2218"),
	"Review":   lipgloss.Color("#2a2618"),
	"Support":  lipgloss.Color("#222228"),
}

// groupLightColor maps group name → lighter/pastel variant for detail text.
var groupLightColor = map[string]color.Color{
	"Capture":  lipgloss.Color("#f0a0a0"),
	"Organize": lipgloss.Color("#90c0e0"),
	"Plan":     lipgloss.Color("#80d0a0"),
	"Work":     lipgloss.Color("#e0b080"),
	"Review":   lipgloss.Color("#e0d090"),
	"Support":  lipgloss.Color("#9aa5ce"),
}

// rebuildGroupColors sets theme-appropriate group color maps.
func rebuildGroupColors() {
	if activeThemeName == "light" {
		groupBgColor = map[string]color.Color{
			"Capture":  lipgloss.Color("#f5e6e8"),
			"Organize": lipgloss.Color("#e4edf5"),
			"Plan":     lipgloss.Color("#e4f0e8"),
			"Work":     lipgloss.Color("#f5ece4"),
			"Review":   lipgloss.Color("#f5f0e0"),
			"Support":  lipgloss.Color("#eaeaee"),
		}
		groupAccentColor = map[string]color.Color{
			"Capture":  lipgloss.Color("#b91c1c"),
			"Organize": lipgloss.Color("#0e7490"),
			"Plan":     lipgloss.Color("#15803d"),
			"Work":     lipgloss.Color("#c2410c"),
			"Review":   lipgloss.Color("#92400e"),
			"Support":  lipgloss.Color("#4b5563"),
		}
		groupLightColor = map[string]color.Color{
			"Capture":  lipgloss.Color("#9f1239"),
			"Organize": lipgloss.Color("#155e75"),
			"Plan":     lipgloss.Color("#166534"),
			"Work":     lipgloss.Color("#9a3412"),
			"Review":   lipgloss.Color("#78350f"),
			"Support":  lipgloss.Color("#6b7280"),
		}
	} else {
		groupBgColor = map[string]color.Color{
			"Capture":  lipgloss.Color("#2a1f2a"),
			"Organize": lipgloss.Color("#1e2535"),
			"Plan":     lipgloss.Color("#1e2a22"),
			"Work":     lipgloss.Color("#2a2218"),
			"Review":   lipgloss.Color("#2a2618"),
			"Support":  lipgloss.Color("#222228"),
		}
		groupAccentColor = map[string]color.Color{
			"Capture":  AccentCapture,
			"Organize": AccentOrganize,
			"Plan":     AccentPlan,
			"Work":     AccentWork,
			"Review":   AccentReview,
			"Support":  AccentSupport,
		}
		groupLightColor = map[string]color.Color{
			"Capture":  lipgloss.Color("#f0a0a0"),
			"Organize": lipgloss.Color("#90c0e0"),
			"Plan":     lipgloss.Color("#80d0a0"),
			"Work":     lipgloss.Color("#e0b080"),
			"Review":   lipgloss.Color("#e0d090"),
			"Support":  lipgloss.Color("#9aa5ce"),
		}
	}
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

// buildPrefixBreadcrumb builds a human-readable breadcrumb string for multi-level prefixes.
// For "Ft" it returns "FILTER > TYPE"; for "Fas" it returns "FILTER > ATTR > SEVERITY".
// Returns empty string for single-character prefixes.
func buildPrefixBreadcrumb(prefix string) string {
	if len(prefix) <= 1 {
		return ""
	}
	var parts []string
	prevName := ""
	for i := 1; i <= len(prefix); i++ {
		seg := prefix[:i]
		name := prefixCategoryNames[seg]
		if name == "" {
			name = seg
		}
		display := strings.ToUpper(name)
		if prevName != "" && strings.HasPrefix(strings.ToLower(name), strings.ToLower(prevName)+"-") {
			display = strings.ToUpper(name[len(prevName)+1:])
		}
		parts = append(parts, display)
		prevName = name
	}
	return strings.Join(parts, " > ")
}

// RenderPrefixPopup renders a which-key style popup as a bordered overlay box.
// Returns empty string if prefix is empty or not found in hints.
func RenderPrefixPopup(prefix string, hints PrefixHints, width, height int) string {
	return RenderPrefixPopupWithCategory(prefix, "", hints, width, height)
}

// RenderPrefixPopupWithCategory renders a prefix popup with a custom category name.
// If category is empty, it falls back to prefixCategoryNames or the prefix itself.
func RenderPrefixPopupWithCategory(prefix, category string, hints PrefixHints, width, height int) string {
	if prefix == "" {
		return ""
	}

	keyHints, ok := hints[prefix]
	if !ok || len(keyHints) == 0 {
		return ""
	}

	if category == "" {
		category = prefixCategoryNames[prefix]
		if category == "" {
			category = prefix
		}
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	titleKeyStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	titleCatFirstStyle := SuccessBoldStyle
	titleCatRestStyle := NormalStyle

	var title string
	isModifierKey := strings.Contains(prefix, "+")
	if len(prefix) > 1 && !isModifierKey {
		breadcrumb := buildPrefixBreadcrumb(prefix)
		if breadcrumb == "" {
			breadcrumb = strings.ToUpper(category)
		}
		// Render breadcrumb: each segment separated by " > ", last segment bold
		segments := strings.Split(breadcrumb, " > ")
		var renderedParts []string
		for i, seg := range segments {
			if i == len(segments)-1 {
				renderedParts = append(renderedParts, SuccessBoldStyle.Render(seg))
			} else {
				renderedParts = append(renderedParts, MutedStyle.Render(seg))
			}
		}
		crumbStr := strings.Join(renderedParts, MutedStyle.Render(" > "))
		title = titleKeyStyle.Render(prefix) + borderStyle.Render(" → ") + crumbStr
	} else {
		categoryTitle := renderPrefixCategoryTitle(prefix, category, titleCatFirstStyle, titleCatRestStyle)
		title = titleKeyStyle.Render(prefix) + borderStyle.Render(" → ") + categoryTitle
	}

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

func renderPrefixCategoryTitle(prefix, category string, highlightedStyle, baseStyle lipgloss.Style) string {
	if shouldRenderWholePrefixCategoryTitleBold(prefix, category) {
		return baseStyle.Bold(true).Render(category)
	}

	start, end, ok := resolvePrefixCategoryHighlightSpan(prefix, category)
	if !ok {
		return baseStyle.Render(category)
	}

	return baseStyle.Render(category[:start]) +
		highlightedStyle.Render(category[start:end]) +
		baseStyle.Render(category[end:])
}

func shouldRenderWholePrefixCategoryTitleBold(prefix, category string) bool {
	return prefix == ";" && category == "tt"
}

func resolvePrefixCategoryHighlightSpan(prefix, category string) (int, int, bool) {
	mnemonic, ok, usedOverride := resolvePrefixCategoryMnemonic(prefix, category)
	if !ok {
		return 0, 0, false
	}

	if start, end, found := findCaseInsensitiveRuneSpan(category, mnemonic); found {
		return start, end, true
	}

	// If override mnemonic is not present in category, fallback to first rune.
	if usedOverride {
		return firstRuneSpan(category)
	}

	return 0, 0, false
}

func resolvePrefixCategoryMnemonic(prefix, category string) (rune, bool, bool) {
	if mnemonic, ok := prefixCategoryMnemonicOverrides[prefix]; ok {
		return mnemonic, true, true
	}
	for _, r := range category {
		return r, true, false
	}
	return 0, false, false
}

func findCaseInsensitiveRuneSpan(s string, target rune) (int, int, bool) {
	lowerTarget := unicode.ToLower(target)
	for start, r := range s {
		if unicode.ToLower(r) != lowerTarget {
			continue
		}
		_, size := utf8.DecodeRuneInString(s[start:])
		if size <= 0 {
			return 0, 0, false
		}
		return start, start + size, true
	}
	return 0, 0, false
}

func firstRuneSpan(s string) (int, int, bool) {
	if s == "" {
		return 0, 0, false
	}
	_, size := utf8.DecodeRuneInString(s)
	if size <= 0 {
		return 0, 0, false
	}
	return 0, size, true
}

func renderAlignedPrefixHintLines(keyHints []PrefixHint) ([]string, int) {
	layout := measurePrefixHintLineLayout(keyHints)

	contentWidth := 0
	contentLines := make([]string, 0, len(keyHints))
	for _, hint := range keyHints {
		rendered, lineWidth := renderAlignedPrefixHintLine(
			hint,
			layout.maxKeyWidth,
			layout.maxDescWidth,
			layout.maxIconWidth,
			layout.iconColWidth,
			layout.hasDetail,
			layout.hasIcon,
			"",
		)
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
	layout := measurePrefixHintLineLayout(hints)
	maxGroupWidth := maxPipelineGroupWidth()
	groupColWidth := maxGroupWidth + 2 // name + "  " gap
	contentWidth := groupedPrefixHintContentWidth(layout, groupColWidth)
	sorted := sortGroupedPrefixHints(hints)

	var contentLines []string
	currentGroup := "\x00" // sentinel: no group yet
	blankGroupCol := strings.Repeat(" ", groupColWidth)

	for _, hint := range sorted {
		rendered, _ := renderAlignedPrefixHintLine(
			hint,
			layout.maxKeyWidth,
			layout.maxDescWidth,
			layout.maxIconWidth,
			layout.iconColWidth,
			layout.hasDetail,
			layout.hasIcon,
			hint.Group,
		)

		if hint.Group != currentGroup {
			currentGroup = hint.Group
			contentLines = append(contentLines, renderGroupedPrefixHintLine(currentGroup, maxGroupWidth, blankGroupCol, rendered))
			continue
		}
		contentLines = append(contentLines, blankGroupCol+rendered)
	}

	return contentLines, contentWidth
}

type prefixHintLineLayout struct {
	maxKeyWidth    int
	maxDescWidth   int
	maxIconWidth   int
	maxDetailWidth int
	iconColWidth   int
	hasDetail      bool
	hasIcon        bool
}

func measurePrefixHintLineLayout(hints []PrefixHint) prefixHintLineLayout {
	layout := prefixHintLineLayout{}
	for _, hint := range hints {
		layout.maxKeyWidth = max(layout.maxKeyWidth, len(hint.Key))
		layout.maxDescWidth = max(layout.maxDescWidth, len(hint.Description))
		if hint.Detail != "" {
			layout.hasDetail = true
			layout.maxDetailWidth = max(layout.maxDetailWidth, len(hint.Detail))
		}
		if hint.RenderedIcon != "" {
			layout.hasIcon = true
			layout.maxIconWidth = max(layout.maxIconWidth, ansi.StringWidth(hint.RenderedIcon))
		}
	}
	if layout.hasIcon {
		layout.iconColWidth = layout.maxIconWidth + 1
	}
	return layout
}

func maxPipelineGroupWidth() int {
	maxGroupWidth := 0
	for _, group := range groupOrder {
		maxGroupWidth = max(maxGroupWidth, len(group))
	}
	return maxGroupWidth
}

func groupedPrefixHintContentWidth(layout prefixHintLineLayout, groupColWidth int) int {
	contentWidth := groupColWidth + layout.maxKeyWidth + 2 + layout.iconColWidth + layout.maxDescWidth
	if layout.hasDetail {
		contentWidth += 2 + layout.maxDetailWidth
	}
	return contentWidth
}

func sortGroupedPrefixHints(hints []PrefixHint) []PrefixHint {
	sorted := make([]PrefixHint, len(hints))
	copy(sorted, hints)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri, rj := prefixHintGroupRank(sorted[i].Group), prefixHintGroupRank(sorted[j].Group)
		if ri != rj {
			return ri < rj
		}
		ai, aj := prefixHintKeyRank(sorted[i].Key), prefixHintKeyRank(sorted[j].Key)
		if ai != aj {
			return ai < aj
		}
		return sorted[i].Key < sorted[j].Key
	})
	return sorted
}

func prefixHintGroupRank(group string) int {
	if group == "" {
		return -1
	}
	return groupRank[group]
}

func prefixHintKeyRank(key string) int {
	if len(key) == 0 {
		return 1
	}
	switch {
	case key[0] >= 'a' && key[0] <= 'z':
		return 0
	case key[0] >= 'A' && key[0] <= 'Z':
		return 2
	default:
		return 1
	}
}

func renderGroupedPrefixHintLine(group string, maxGroupWidth int, blankGroupCol, rendered string) string {
	if group == "" {
		return blankGroupCol + rendered
	}

	labelStyle := lipgloss.NewStyle()
	if accent, ok := groupAccentColor[group]; ok {
		labelStyle = labelStyle.Foreground(accent)
	}
	if bg, ok := groupBgColor[group]; ok {
		labelStyle = labelStyle.Background(bg)
	}
	label := labelStyle.Render(fmt.Sprintf(" %-*s ", maxGroupWidth, group))
	return label + rendered
}

func renderAlignedPrefixHintLine(hint PrefixHint, maxKeyWidth, maxDescWidth, maxIconWidth, iconColWidth int, hasDetail, hasIcon bool, group string) (string, int) {
	keyStyle := PrefixKeyStyle
	descStyle := PrefixDescStyle
	detailStyle := MutedStyle
	if hint.Disabled {
		keyStyle = MutedStyle
		descStyle = MutedStyle
		detailStyle = MutedStyle
	} else if hint.Danger {
		keyStyle = ErrorStatusStyle
		descStyle = ErrorStatusStyle
		detailStyle = ErrorStatusStyle
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
	footer := MutedStyle.Render("Esc to cancel")
	return renderTitledBox(content, innerWidth, title, footer, borderStyle)
}
