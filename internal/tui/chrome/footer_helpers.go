package chrome

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/tui/keymap"
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
//
// DropOrphan refuses a last row holding one optional entry alone: that entry is
// shed instead, so a wrapped footer never spends a whole row on a single
// convenience. A required entry is never shed for it, and a spec that fits one
// row is unaffected.
type GuidanceSpec struct {
	Hints        []GuidanceHint
	Supplemental string
	Rows         int
	DropOrphan   bool
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
			NewGuidanceHint("?/Esc close syntax", 0, true),
		}
		spec.Supplemental = fmt.Sprintf("%d%%", scrollPercent)
	case helpExpanded:
		spec.Hints = []GuidanceHint{
			NewGuidanceHint("Enter create", 0, true),
			NewGuidanceHint("Ctrl+E create & edit", 1, false),
			CapturePasteHint(),
			NewGuidanceHint("?/Esc close syntax", 0, true),
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
