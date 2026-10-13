package chrome

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// GuidanceHint is one complete key/action entry in contextual guidance.
// Lower Priority values are retained before higher values. Required hints are
// reserved before any optional hint is considered, regardless of priority.
//
// Group opens a named division ahead of this entry. It rides the entry rather
// than standing as a hint of its own so a heading and the first entry under it
// are packed, fitted, and dropped as one thing: a heading the packer kept while
// its only entry did not fit would name a division the row does not show.
type GuidanceHint struct {
	Key      string
	Label    string
	Group    string
	Priority int
	Required bool
	Dimmed   bool // The action currently has no effect; it remains discoverable.
}

// GuidanceSpec describes the persistent controls for one interaction state.
// Hints retain declaration order when rendered. Supplemental is optional
// state data (for example a scroll percentage), not an action. Rows is clamped
// to the product's guidance budget of one to MaxGuidanceRows; zero means one
// row.
type GuidanceSpec struct {
	Hints        []GuidanceHint
	Supplemental string
	Rows         int
}

// MaxGuidanceRows is the tallest guidance block the packer will lay out. Only a
// pending chord asks for it: its entries are the whole answer to the prefix
// just pressed, so shedding one hides a key rather than a convenience.
const MaxGuidanceRows = 3

// capturePastePriority ranks `Ctrl+V paste` behind every other optional capture
// hint, the composer's own included: the terminal's paste already works, so it
// is the entry a narrow footer can best afford to lose.
const capturePastePriority = 1000

// CapturePasteHint is that entry, for the capture states built outside
// CaptureGuidanceSpec.
func CapturePasteHint() GuidanceHint {
	return NewGuidanceHint("Ctrl+V paste", capturePastePriority, false)
}

// CaptureGuidanceSpec selects the controls for the composer's current
// interaction. Capture is always active input, so every state receives the
// two-row budget and marks its essential completion/cancellation controls as
// required.
func CaptureGuidanceSpec(ConfirmDiscard, autocompleteActive, helpExpanded, syntaxOverflow bool, scrollPercent int, hasDraft bool) GuidanceSpec {
	var spec GuidanceSpec
	switch {
	case ConfirmDiscard:
		spec = DiscardGuidanceSpec()
	case autocompleteActive:
		spec.Hints = []GuidanceHint{
			NewGuidanceHint("Tab/Enter accept", 0, true),
			NewGuidanceHint("Ctrl+J/K navigate", 1, false),
			NewGuidanceHint("Esc close suggestions", 0, true),
		}
	case helpExpanded && syntaxOverflow:
		spec.Hints = []GuidanceHint{
			NewGuidanceHint("Enter create", 0, true),
			NewGuidanceHint("Ctrl+E create & edit", 1, false),
			CapturePasteHint(),
			NewGuidanceHint("Ctrl+J/K scroll", 2, false),
			NewGuidanceHint("PgUp/PgDn page", 3, false),
			NewGuidanceHint("? close syntax", 0, true),
			NewGuidanceHint("Esc close syntax", 0, true),
		}
		spec.Supplemental = fmt.Sprintf("%d%%", scrollPercent)
	case helpExpanded:
		spec.Hints = []GuidanceHint{
			NewGuidanceHint("Enter create", 0, true),
			NewGuidanceHint("Ctrl+E create & edit", 1, false),
			CapturePasteHint(),
			NewGuidanceHint("? close syntax", 0, true),
			NewGuidanceHint("Esc close syntax", 0, true),
		}
	default:
		spec.Hints = []GuidanceHint{
			NewGuidanceHint("Enter create", 0, true),
			NewGuidanceHint("Ctrl+E create & edit", 1, false),
			CapturePasteHint(),
			NewGuidanceHint("? syntax", 2, false),
			NewGuidanceHint(DraftEscapeHint(hasDraft), 0, true),
		}
	}
	spec.Rows = 2
	return spec
}

// DraftEscapeHint names what Esc does on a surface holding an editable draft:
// unsaved text raises the discard question, an untouched draft leaves outright.
// One key, two acts, so the entry is decided here rather than spelled again at
// each editor; a surface that can hold no draft passes false and keeps cancel.
func DraftEscapeHint(hasDraft bool) string {
	if hasDraft {
		return "Esc discard"
	}
	return "Esc cancel"
}

// DiscardGuidanceSpec is the chrome for the unsaved-work question alone. It is
// not ConfirmationGuidanceSpec because declining that question keeps the draft
// and stays in the editor, which "cancel" over an open editor reads as the
// opposite of.
func DiscardGuidanceSpec() GuidanceSpec {
	return GuidanceSpec{
		Hints: RequiredGuidanceHints("Y discard", "N/Esc keep editing"),
		Rows:  2,
	}
}

// ConfirmationGuidanceSpec is the canonical destructive-confirmation chrome.
// Confirmation overlays block every unrelated key, so both outcomes are
// required and keep the two-row interaction budget at narrow widths.
func ConfirmationGuidanceSpec() GuidanceSpec {
	return GuidanceSpec{
		Hints: RequiredGuidanceHints("y confirm", "n/Esc cancel"),
		Rows:  2,
	}
}

func RequiredGuidanceHints(entries ...string) []GuidanceHint {
	return guidanceHintsFromEntries(true, 0, entries...)
}

func OptionalGuidanceHints(priority int, entries ...string) []GuidanceHint {
	return guidanceHintsFromEntries(false, priority, entries...)
}

func NewGuidanceHint(entry string, priority int, required bool) GuidanceHint {
	key, label := splitGuidanceEntry(entry)
	return GuidanceHint{Key: key, Label: label, Priority: priority, Required: required}
}

// GuidanceSpecFromKeyMap adapts live gated binding metadata without copying
// gate or visibility decisions into a surface renderer.
func GuidanceSpecFromKeyMap(km *keymap.KeyMap, rows int) (GuidanceSpec, bool) {
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

func GuidanceHintText(hint GuidanceHint) string {
	return strings.TrimSpace(strings.TrimSpace(hint.Key) + " " + strings.TrimSpace(hint.Label))
}

// guidanceSegment is one packed footer entry. Key and Label carry a keystroke
// entry whose two halves are colored apart at render time; Verbatim carries
// supplemental state text that names no keystroke and stays fully muted.
type guidanceSegment struct {
	Key      string
	Label    string
	Group    string
	Verbatim string
	Dimmed   bool
}

func (segment guidanceSegment) text() string {
	if segment.Verbatim != "" {
		return segment.Verbatim
	}
	entry := GuidanceHintText(GuidanceHint{Key: segment.Key, Label: segment.Label})
	if group := strings.TrimSpace(segment.Group); group != "" {
		return strings.TrimSpace(group + " " + entry)
	}
	return entry
}

// FitToWidth clips one segment for the narrow fallback path, keeping the
// keystroke intact so the entry stays actionable and only its label loses
// characters. A key too wide to pair with any label degrades to verbatim text.
func (segment guidanceSegment) fitToWidth(width int) guidanceSegment {
	text := segment.text()
	width = max(width, 1)
	if lipgloss.Width(text) <= width {
		return segment
	}
	key := strings.TrimSpace(segment.Key)
	group := strings.TrimSpace(segment.Group)
	head := lipgloss.Width(key)
	if group != "" {
		head += lipgloss.Width(group) + 1
	}
	if segment.Verbatim == "" && key != "" && head+1 < width {
		label := textfit.TruncateToVisualWidth(strings.TrimSpace(segment.Label), width-head-1)
		return guidanceSegment{Key: key, Label: label, Group: group, Dimmed: segment.Dimmed}
	}
	return guidanceSegment{Verbatim: textfit.TruncateToVisualWidth(text, width), Dimmed: segment.Dimmed}
}

func guidanceLineText(segments []guidanceSegment) string {
	texts := make([]string, 0, len(segments))
	for _, segment := range segments {
		texts = append(texts, segment.text())
	}
	return strings.Join(texts, FooterHelpSeparator)
}

// GuidanceLines is the plain-text projection of the packed guidance. Packing
// stays plain so every width decision is made before any styling exists.
func GuidanceLines(spec GuidanceSpec, width int) []string {
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
	rows := min(max(spec.Rows, 1), MaxGuidanceRows)

	type candidate struct {
		index   int
		hint    GuidanceHint
		segment guidanceSegment
	}
	candidates := make([]candidate, 0, len(spec.Hints))
	selected := make(map[int]bool, len(spec.Hints))
	for index, hint := range spec.Hints {
		segment := guidanceSegment{
			Key:    strings.TrimSpace(hint.Key),
			Label:  strings.TrimSpace(hint.Label),
			Group:  strings.TrimSpace(hint.Group),
			Dimmed: hint.Dimmed,
		}
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
		candidate := guidanceLineText(lines[last]) + FooterHelpSeparator + text
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
		candidate := guidanceLineText(lines[last]) + FooterHelpSeparator + segment.text()
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
	if segment.Dimmed {
		return theme.DimStyle.Render(segment.text())
	}
	if segment.Verbatim != "" {
		return theme.HelpStyle.Render(segment.Verbatim)
	}
	key := strings.TrimSpace(segment.Key)
	label := strings.TrimSpace(segment.Label)
	// The heading names a division rather than a keystroke, so it takes the
	// action weight and leaves the key the only thing that scans as pressable.
	var heading string
	if group := strings.TrimSpace(segment.Group); group != "" {
		heading = theme.HelpStyle.Render(group + " ")
	}
	switch {
	case key == "":
		return heading + theme.HelpStyle.Render(label)
	case label == "":
		return heading + theme.HelpKeyStyle.Render(key)
	}
	return heading + theme.HelpKeyStyle.Render(key) + theme.HelpStyle.Render(" "+label)
}

// RenderGuidanceLines styles the packed guidance one segment at a time. Styling
// happens after packing, so the visible width matches the plain projection.
func RenderGuidanceLines(spec GuidanceSpec, width int) []string {
	if width <= 0 {
		width = 80
	}
	segmentLines := guidanceSegmentLines(spec, width)
	if len(segmentLines) == 0 {
		return nil
	}
	separator := theme.HelpStyle.Render(FooterHelpSeparator)
	lines := make([]string, 0, len(segmentLines))
	for _, segments := range segmentLines {
		styled := make([]string, 0, len(segments))
		for _, segment := range segments {
			styled = append(styled, renderGuidanceSegment(segment))
		}
		lines = append(lines, textfit.TruncateToVisualWidth(strings.Join(styled, separator), width))
	}
	return lines
}

// RenderGuidance is the sole structured guidance renderer used by shell-owned
// contextual controls and pending chords. It emits one divider plus at most the
// spec's one- or two-row budget.
func RenderGuidance(spec GuidanceSpec, width int, accent ...color.Color) string {
	var dividerOpts []DividerOption
	if len(accent) > 0 && accent[0] != nil {
		dividerOpts = append(dividerOpts, WithDividerColor(accent[0]))
	}
	return renderGuidanceWithSeparator(spec, width, RenderHorizontalDivider(width, dividerOpts...))
}

// renderGuidanceWithSeparator keeps shared guidance packing while allowing a
// shell-owned footer to supply a divider containing multiple styled runs.
func renderGuidanceWithSeparator(spec GuidanceSpec, width int, separator string) string {
	if width <= 0 {
		width = 80
	}
	lines := RenderGuidanceLines(spec, width)
	if len(lines) == 0 {
		return ""
	}
	return separator + "\n" + strings.Join(lines, "\n")
}

// RenderGuidanceText renders structured guidance without a divider. Inline
// overlays use it inside their own bordered frame, where a second separator
// would duplicate the frame's visual boundary.
func RenderGuidanceText(spec GuidanceSpec, width int) string {
	return strings.Join(RenderGuidanceLines(spec, width), "\n")
}

// RequiredActionGuidanceSpec builds the common two-row contract for active
// editors: the commit and cancel exits are reserved before optional controls.
func RequiredActionGuidanceSpec(commit string, optional ...string) GuidanceSpec {
	return RequiredExitGuidanceSpec(commit, "Esc cancel", optional...)
}

// RequiredExitGuidanceSpec is that same contract where Esc does something other
// than cancel outright — a draft-bearing editor raising the discard question,
// for one — so the exit entry is named by the caller that knows which act the
// key will reach.
func RequiredExitGuidanceSpec(commit, exit string, optional ...string) GuidanceSpec {
	return GuidanceSpec{
		Hints: append(RequiredGuidanceHints(commit, exit), OptionalGuidanceHints(10, optional...)...),
		Rows:  2,
	}
}

func EscapeGuidanceSpec() GuidanceSpec {
	return GuidanceSpec{Hints: OptionalGuidanceHints(0, "Esc back", "? help"), Rows: 1}
}

// JoinGuidanceContext joins the supplemental fragments a surface wants under
// its hints into the one separator the guidance row uses, dropping the blanks.
// It lives here rather than beside either caller because supplemental context
// is guidance vocabulary: Notes appends the cursor's location to it and Help
// appends its empty-result note, and both must read as one line.
func JoinGuidanceContext(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return strings.Join(values, " · ")
}
