package tui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// captureGuidanceSpec selects the controls for the composer's current
// interaction. Capture is always active input, so every state receives the
// two-row budget and marks its essential completion/cancellation controls as
// required.
func captureGuidanceSpec(confirmDiscard, autocompleteActive, helpExpanded, syntaxOverflow bool, scrollPercent int) GuidanceSpec {
	var spec GuidanceSpec
	switch {
	case confirmDiscard:
		spec.Hints = requiredGuidanceHints("Y discard", "N/Esc keep editing")
	case autocompleteActive:
		spec.Hints = []GuidanceHint{
			newGuidanceHint("Tab/Enter accept", 0, true),
			newGuidanceHint("Ctrl+J/K navigate", 1, false),
			newGuidanceHint("Esc dismiss", 0, true),
		}
	case helpExpanded && syntaxOverflow:
		spec.Hints = []GuidanceHint{
			newGuidanceHint("Enter create", 0, true),
			newGuidanceHint("Ctrl+D create & edit", 1, false),
			newGuidanceHint("Ctrl+V paste", 2, false),
			newGuidanceHint("Ctrl+J/K scroll", 3, false),
			newGuidanceHint("PgUp/PgDn page", 4, false),
			newGuidanceHint("? close syntax", 0, true),
			newGuidanceHint("Esc close syntax", 0, true),
		}
		spec.Supplemental = fmt.Sprintf("%d%%", scrollPercent)
	case helpExpanded:
		spec.Hints = []GuidanceHint{
			newGuidanceHint("Enter create", 0, true),
			newGuidanceHint("Ctrl+D create & edit", 1, false),
			newGuidanceHint("Ctrl+V paste", 2, false),
			newGuidanceHint("? close syntax", 0, true),
			newGuidanceHint("Esc close syntax", 0, true),
		}
	default:
		spec.Hints = []GuidanceHint{
			newGuidanceHint("Enter create", 0, true),
			newGuidanceHint("Ctrl+D create & edit", 1, false),
			newGuidanceHint("Ctrl+V paste", 2, false),
			newGuidanceHint("? syntax", 3, false),
			newGuidanceHint("Esc cancel", 0, true),
		}
	}
	spec.Rows = 2
	return spec
}

// confirmationGuidanceSpec is the canonical destructive-confirmation chrome.
// Confirmation overlays block every unrelated key, so both outcomes are
// required and keep the two-row interaction budget at narrow widths.
func confirmationGuidanceSpec() GuidanceSpec {
	return GuidanceSpec{
		Hints: requiredGuidanceHints("y confirm", "n/Esc cancel"),
		Rows:  2,
	}
}

func requiredGuidanceHints(entries ...string) []GuidanceHint {
	return guidanceHintsFromEntries(true, 0, entries...)
}

func optionalGuidanceHints(priority int, entries ...string) []GuidanceHint {
	return guidanceHintsFromEntries(false, priority, entries...)
}

func newGuidanceHint(entry string, priority int, required bool) GuidanceHint {
	key, label := splitGuidanceEntry(entry)
	return GuidanceHint{Key: key, Label: label, Priority: priority, Required: required}
}

// guidanceSpecFromKeyMap adapts live gated binding metadata without copying
// gate or visibility decisions into a surface renderer.
func guidanceSpecFromKeyMap(km *keymap.KeyMap, rows int) (GuidanceSpec, bool) {
	footerHints := km.FooterHints()
	if len(footerHints) == 0 {
		return GuidanceSpec{}, false
	}
	spec := GuidanceSpec{Rows: rows, Hints: make([]GuidanceHint, 0, len(footerHints))}
	for _, hint := range footerHints {
		spec.Hints = append(spec.Hints, GuidanceHint{
			Key:      hint.Key,
			Label:    hint.Label,
			Priority: hint.Rank,
			Required: hint.Required,
		})
	}
	return spec, true
}

func pendingChordGuidanceSpec(prefix string, hints []ChordHint) GuidanceSpec {
	spec := GuidanceSpec{Rows: 2}
	if prefix = strings.TrimSpace(prefix); prefix != "" {
		spec.Hints = append(spec.Hints, GuidanceHint{Key: prefix + "…"})
	}
	for index, hint := range hints {
		spec.Hints = append(spec.Hints, GuidanceHint{
			Key:      hint.Key,
			Label:    hint.Label,
			Priority: index + 1,
		})
	}
	spec.Hints = append(spec.Hints, GuidanceHint{
		Key:      "Esc",
		Label:    "cancel",
		Required: true,
	})
	return spec
}

func guidanceHintsFromEntries(required bool, basePriority int, entries ...string) []GuidanceHint {
	hints := make([]GuidanceHint, 0, len(entries))
	for index, entry := range entries {
		key, label := splitGuidanceEntry(entry)
		if key == "" && label == "" {
			continue
		}
		hints = append(hints, GuidanceHint{
			Key:      key,
			Label:    label,
			Priority: basePriority + index,
			Required: required,
		})
	}
	return hints
}

func splitGuidanceEntry(entry string) (string, string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", ""
	}
	if index := strings.IndexAny(entry, " \t"); index >= 0 {
		return entry[:index], strings.TrimSpace(entry[index:])
	}
	return entry, ""
}

func guidanceHintText(hint GuidanceHint) string {
	return strings.TrimSpace(strings.TrimSpace(hint.Key) + " " + strings.TrimSpace(hint.Label))
}

// guidanceLines selects and packs complete entries within the spec's row
// budget. Required entries are reserved first. Optional entries are considered
// by priority while successful entries retain declaration order; after one
// optional entry cannot fit, lower-priority entries are not promoted. The
// supplemental value is attempted last and therefore drops first under pressure.
func guidanceLines(spec GuidanceSpec, width int) []string {
	if width <= 0 {
		width = 80
	}
	rows := 1
	if spec.Rows >= 2 {
		rows = 2
	}

	type candidate struct {
		index int
		hint  GuidanceHint
		text  string
	}
	candidates := make([]candidate, 0, len(spec.Hints))
	selected := make(map[int]bool, len(spec.Hints))
	for index, hint := range spec.Hints {
		text := guidanceHintText(hint)
		if text == "" {
			continue
		}
		entryCandidate := candidate{index: index, hint: hint, text: text}
		candidates = append(candidates, entryCandidate)
		if hint.Required {
			selected[index] = true
		}
	}

	selectedEntries := func(extra string) []string {
		entries := make([]string, 0, len(selected)+1)
		for _, candidate := range candidates {
			if selected[candidate.index] {
				entries = append(entries, candidate.text)
			}
		}
		if extra = strings.TrimSpace(extra); extra != "" {
			entries = append(entries, extra)
		}
		return entries
	}

	lines, requiredFit := packGuidanceEntries(selectedEntries(""), width, rows)
	if !requiredFit {
		return fitRequiredGuidanceEntries(selectedEntries(""), width, rows)
	}

	optional := make([]candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.hint.Required {
			optional = append(optional, candidate)
		}
	}
	sort.SliceStable(optional, func(i, j int) bool {
		return optional[i].hint.Priority < optional[j].hint.Priority
	})
	for _, candidate := range optional {
		selected[candidate.index] = true
		packed, ok := packGuidanceEntries(selectedEntries(""), width, rows)
		if !ok {
			delete(selected, candidate.index)
			break
		}
		lines = packed
	}

	if supplemental := strings.TrimSpace(spec.Supplemental); supplemental != "" {
		if packed, ok := packGuidanceEntries(selectedEntries(supplemental), width, rows); ok {
			lines = packed
		}
	}
	return lines
}

func packGuidanceEntries(entries []string, width, maxLines int) ([]string, bool) {
	if len(entries) == 0 || width <= 0 || maxLines <= 0 {
		return nil, len(entries) == 0
	}
	lines := make([]string, 0, maxLines)
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" || lipgloss.Width(entry) > width {
			return nil, false
		}
		if len(lines) == 0 {
			lines = append(lines, entry)
			continue
		}
		candidate := lines[len(lines)-1] + footerHelpSeparator + entry
		if lipgloss.Width(candidate) <= width {
			lines[len(lines)-1] = candidate
			continue
		}
		if len(lines) == maxLines {
			return nil, false
		}
		lines = append(lines, entry)
	}
	return lines, true
}

func fitRequiredGuidanceEntries(entries []string, width, rows int) []string {
	lines := make([]string, 0, rows)
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		entry = fitPlainLineToWidth(entry, width)
		if len(lines) == 0 {
			lines = append(lines, entry)
			continue
		}
		candidate := lines[len(lines)-1] + footerHelpSeparator + entry
		if lipgloss.Width(candidate) <= width {
			lines[len(lines)-1] = candidate
			continue
		}
		if len(lines) == rows {
			break
		}
		lines = append(lines, entry)
	}
	return lines
}

func fitPlainLineToWidth(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	return truncateToVisualWidth(line, max(width, 1))
}

// renderGuidance is the sole structured guidance renderer used by shell-owned
// contextual controls and pending chords. It emits one divider plus at most the
// spec's one- or two-row budget.
func renderGuidance(spec GuidanceSpec, width int, accent ...color.Color) string {
	if width <= 0 {
		width = 80
	}
	lines := guidanceLines(spec, width)
	if len(lines) == 0 {
		return ""
	}
	var separatorStyle lipgloss.Style
	if len(accent) > 0 && accent[0] != nil {
		separatorStyle = lipgloss.NewStyle().Foreground(accent[0])
	} else {
		separatorStyle = MutedStyle
	}
	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		styled = append(styled, HelpStyle.Render(fitPlainLineToWidth(line, width)))
	}
	return separatorStyle.Render(strings.Repeat("─", width)) + "\n" + strings.Join(styled, "\n")
}

// renderGuidanceText renders structured guidance without a divider. Inline
// overlays use it inside their own bordered frame, where a second separator
// would duplicate the frame's visual boundary.
func renderGuidanceText(spec GuidanceSpec, width int) string {
	lines := guidanceLines(spec, width)
	if len(lines) == 0 {
		return ""
	}
	return HelpStyle.Render(strings.Join(lines, "\n"))
}

// requiredActionGuidanceSpec builds the common two-row contract for active
// editors: the commit and cancel exits are reserved before optional controls.
func requiredActionGuidanceSpec(commit string, optional ...string) GuidanceSpec {
	return GuidanceSpec{
		Hints: append(requiredGuidanceHints(commit, "Esc cancel"), optionalGuidanceHints(10, optional...)...),
		Rows:  2,
	}
}

func escapeGuidanceSpec() GuidanceSpec {
	return GuidanceSpec{Hints: optionalGuidanceHints(0, "Esc back", "? help"), Rows: 1}
}
