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
			newGuidanceHint("Ctrl+E create & edit", 1, false),
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
			newGuidanceHint("Ctrl+E create & edit", 1, false),
			newGuidanceHint("Ctrl+V paste", 2, false),
			newGuidanceHint("? close syntax", 0, true),
			newGuidanceHint("Esc close syntax", 0, true),
		}
	default:
		spec.Hints = []GuidanceHint{
			newGuidanceHint("Enter create", 0, true),
			newGuidanceHint("Ctrl+E create & edit", 1, false),
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

// guidanceSegment is one packed footer entry. Key and Label carry a keystroke
// entry whose two halves are colored apart at render time; Verbatim carries
// supplemental state text that names no keystroke and stays fully muted.
type guidanceSegment struct {
	Key      string
	Label    string
	Verbatim string
}

func (segment guidanceSegment) text() string {
	if segment.Verbatim != "" {
		return segment.Verbatim
	}
	return guidanceHintText(GuidanceHint{Key: segment.Key, Label: segment.Label})
}

// fitToWidth clips one segment for the narrow fallback path, keeping the
// keystroke intact so the entry stays actionable and only its label loses
// characters. A key too wide to pair with any label degrades to verbatim text.
func (segment guidanceSegment) fitToWidth(width int) guidanceSegment {
	text := segment.text()
	width = max(width, 1)
	if lipgloss.Width(text) <= width {
		return segment
	}
	key := strings.TrimSpace(segment.Key)
	if segment.Verbatim == "" && key != "" && lipgloss.Width(key)+1 < width {
		label := truncateToVisualWidth(strings.TrimSpace(segment.Label), width-lipgloss.Width(key)-1)
		return guidanceSegment{Key: key, Label: label}
	}
	return guidanceSegment{Verbatim: truncateToVisualWidth(text, width)}
}

func guidanceLineText(segments []guidanceSegment) string {
	texts := make([]string, 0, len(segments))
	for _, segment := range segments {
		texts = append(texts, segment.text())
	}
	return strings.Join(texts, footerHelpSeparator)
}

// guidanceLines is the plain-text projection of the packed guidance. Packing
// stays plain so every width decision is made before any styling exists.
func guidanceLines(spec GuidanceSpec, width int) []string {
	segmentLines := guidanceSegmentLines(spec, width)
	if len(segmentLines) == 0 {
		return nil
	}
	lines := make([]string, 0, len(segmentLines))
	for _, segments := range segmentLines {
		lines = append(lines, guidanceLineText(segments))
	}
	return lines
}

// guidanceSegmentLines selects and packs complete entries within the spec's row
// budget. Required entries are reserved first. Optional entries are considered
// by priority while successful entries retain declaration order; after one
// optional entry cannot fit, lower-priority entries are not promoted. The
// supplemental value is attempted last and therefore drops first under pressure.
func guidanceSegmentLines(spec GuidanceSpec, width int) [][]guidanceSegment {
	if width <= 0 {
		width = 80
	}
	rows := 1
	if spec.Rows >= 2 {
		rows = 2
	}

	type candidate struct {
		index   int
		hint    GuidanceHint
		segment guidanceSegment
	}
	candidates := make([]candidate, 0, len(spec.Hints))
	selected := make(map[int]bool, len(spec.Hints))
	for index, hint := range spec.Hints {
		segment := guidanceSegment{Key: strings.TrimSpace(hint.Key), Label: strings.TrimSpace(hint.Label)}
		if segment.text() == "" {
			continue
		}
		entryCandidate := candidate{index: index, hint: hint, segment: segment}
		candidates = append(candidates, entryCandidate)
		if hint.Required {
			selected[index] = true
		}
	}

	selectedSegments := func(extra string) []guidanceSegment {
		segments := make([]guidanceSegment, 0, len(selected)+1)
		for _, candidate := range candidates {
			if selected[candidate.index] {
				segments = append(segments, candidate.segment)
			}
		}
		if extra = strings.TrimSpace(extra); extra != "" {
			segments = append(segments, guidanceSegment{Verbatim: extra})
		}
		return segments
	}

	lines, requiredFit := packGuidanceSegments(selectedSegments(""), width, rows)
	if !requiredFit {
		return fitRequiredGuidanceSegments(selectedSegments(""), width, rows)
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
		packed, ok := packGuidanceSegments(selectedSegments(""), width, rows)
		if !ok {
			delete(selected, candidate.index)
			break
		}
		lines = packed
	}

	if supplemental := strings.TrimSpace(spec.Supplemental); supplemental != "" {
		if packed, ok := packGuidanceSegments(selectedSegments(supplemental), width, rows); ok {
			lines = packed
		}
	}
	return lines
}

func packGuidanceSegments(segments []guidanceSegment, width, maxLines int) ([][]guidanceSegment, bool) {
	if len(segments) == 0 || width <= 0 || maxLines <= 0 {
		return nil, len(segments) == 0
	}
	lines := make([][]guidanceSegment, 0, maxLines)
	for _, segment := range segments {
		text := segment.text()
		if text == "" || lipgloss.Width(text) > width {
			return nil, false
		}
		if len(lines) == 0 {
			lines = append(lines, []guidanceSegment{segment})
			continue
		}
		last := len(lines) - 1
		candidate := guidanceLineText(lines[last]) + footerHelpSeparator + text
		if lipgloss.Width(candidate) <= width {
			lines[last] = append(lines[last], segment)
			continue
		}
		if len(lines) == maxLines {
			return nil, false
		}
		lines = append(lines, []guidanceSegment{segment})
	}
	return lines, true
}

func fitRequiredGuidanceSegments(segments []guidanceSegment, width, rows int) [][]guidanceSegment {
	lines := make([][]guidanceSegment, 0, rows)
	for _, segment := range segments {
		segment = segment.fitToWidth(width)
		if segment.text() == "" {
			continue
		}
		if len(lines) == 0 {
			lines = append(lines, []guidanceSegment{segment})
			continue
		}
		last := len(lines) - 1
		candidate := guidanceLineText(lines[last]) + footerHelpSeparator + segment.text()
		if lipgloss.Width(candidate) <= width {
			lines[last] = append(lines[last], segment)
			continue
		}
		if len(lines) == rows {
			break
		}
		lines = append(lines, []guidanceSegment{segment})
	}
	return lines
}

// renderGuidanceSegment colors the keystroke apart from its action so the key
// carries the scan weight. Supplemental state text names no key and stays muted.
func renderGuidanceSegment(segment guidanceSegment) string {
	if segment.Verbatim != "" {
		return HelpStyle.Render(segment.Verbatim)
	}
	key := strings.TrimSpace(segment.Key)
	label := strings.TrimSpace(segment.Label)
	switch {
	case key == "":
		return HelpStyle.Render(label)
	case label == "":
		return HelpKeyStyle.Render(key)
	}
	return HelpKeyStyle.Render(key) + HelpStyle.Render(" "+label)
}

// renderGuidanceLines styles the packed guidance one segment at a time. Styling
// happens after packing, so the visible width matches the plain projection.
func renderGuidanceLines(spec GuidanceSpec, width int) []string {
	if width <= 0 {
		width = 80
	}
	segmentLines := guidanceSegmentLines(spec, width)
	if len(segmentLines) == 0 {
		return nil
	}
	separator := HelpStyle.Render(footerHelpSeparator)
	lines := make([]string, 0, len(segmentLines))
	for _, segments := range segmentLines {
		styled := make([]string, 0, len(segments))
		for _, segment := range segments {
			styled = append(styled, renderGuidanceSegment(segment))
		}
		lines = append(lines, truncateToVisualWidth(strings.Join(styled, separator), width))
	}
	return lines
}

// renderGuidance is the sole structured guidance renderer used by shell-owned
// contextual controls and pending chords. It emits one divider plus at most the
// spec's one- or two-row budget.
func renderGuidance(spec GuidanceSpec, width int, accent ...color.Color) string {
	var dividerOpts []DividerOption
	if len(accent) > 0 && accent[0] != nil {
		dividerOpts = append(dividerOpts, WithDividerColor(accent[0]))
	}
	return renderGuidanceWithSeparator(spec, width, renderHorizontalDivider(width, dividerOpts...))
}

// renderGuidanceWithSeparator keeps shared guidance packing while allowing a
// shell-owned footer to supply a divider containing multiple styled runs.
func renderGuidanceWithSeparator(spec GuidanceSpec, width int, separator string) string {
	if width <= 0 {
		width = 80
	}
	lines := renderGuidanceLines(spec, width)
	if len(lines) == 0 {
		return ""
	}
	return separator + "\n" + strings.Join(lines, "\n")
}

// renderGuidanceText renders structured guidance without a divider. Inline
// overlays use it inside their own bordered frame, where a second separator
// would duplicate the frame's visual boundary.
func renderGuidanceText(spec GuidanceSpec, width int) string {
	return strings.Join(renderGuidanceLines(spec, width), "\n")
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
