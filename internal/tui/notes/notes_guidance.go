package notes

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

// ContextualGuidance returns the controls for the Notes interaction that owns
// input right now. Modal commit/cancel controls are required and receive the
// two-row interaction budget; ordinary browse and causal empty states stay on
// one row.
func (m *NotesViewModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	if m.LocationPicker != nil {
		return m.LocationPicker.guidanceSpec()
	}

	switch {
	case m.confirmDiscard:
		return chrome.GuidanceSpec{
			Hints: chrome.RequiredGuidanceHints("Y discard", "N/Esc keep editing"),
			Rows:  2,
		}, true
	case m.Confirm.IsActive():
		return chrome.GuidanceSpec{
			Hints: chrome.RequiredGuidanceHints("y delete", "n/Esc cancel"),
			Rows:  2,
		}, true
	case m.renaming:
		return chrome.GuidanceSpec{
			Hints:        chrome.RequiredGuidanceHints("Enter save", "Esc cancel"),
			Supplemental: m.currentLocationGuidance(),
			Rows:         2,
		}, true
	case m.searchEditing:
		supplemental := ""
		if len(m.VisibleRows) == 0 {
			supplemental = "no matches"
		}
		hints := append(chrome.RequiredGuidanceHints("Enter apply search", "Esc cancel edit"),
			chrome.OptionalGuidanceHints(10, "Ctrl+J/K navigate")...)
		hints = append(hints, m.previewPagingGuidance(11)...)
		return chrome.GuidanceSpec{
			Hints:        hints,
			Supplemental: supplemental,
			Rows:         2,
		}, true
	}

	if len(m.VisibleRows) == 0 {
		return m.emptyGuidance()
	}

	spec, ok := chrome.GuidanceSpecFromKeyMap(m.BuildKeyMap(), 1)
	if !ok {
		return chrome.GuidanceSpec{}, false
	}
	if query := strings.TrimSpace(m.SearchApplied); query != "" {
		spec.Hints = append(chrome.RequiredGuidanceHints("Esc clear search"), spec.Hints...)
		spec.Supplemental = fmt.Sprintf("search: %s", query)
	}
	spec.Hints = append(spec.Hints, m.previewPagingGuidance(12)...)
	spec.Supplemental = chrome.JoinGuidanceContext(spec.Supplemental, m.currentLocationGuidance())
	return spec, true
}

// previewPagingGuidance advertises Ctrl+F/B only where the press would actually
// move something: the pane is on screen and the note under the cursor draws a
// scrollable body rather than a parse warning. It is never required and ranks
// below every key in the keymap, so it is the first hint a narrow footer sheds
// and it can never displace search apply/cancel or the Esc recovery those states
// owe the reader. The keymap entry stays unranked because the footer may only
// rank keys the Notes strategy binds; this is the same gate, spelled where a
// conditional hint belongs.
func (m *NotesViewModel) previewPagingGuidance(priority int) []chrome.GuidanceHint {
	if !m.notesPreviewPageable() {
		return nil
	}
	return chrome.OptionalGuidanceHints(priority, "Ctrl+F/B preview")
}

func (m *NotesViewModel) emptyGuidance() (chrome.GuidanceSpec, bool) {
	if m.snapshotError != nil {
		return chrome.GuidanceSpec{
			Hints: chrome.RequiredGuidanceHints("Esc close"),
			Rows:  1,
		}, true
	}
	if query := strings.TrimSpace(m.SearchApplied); query != "" {
		return chrome.GuidanceSpec{
			Hints: append(chrome.RequiredGuidanceHints("Esc clear search"),
				chrome.OptionalGuidanceHints(10, "/ edit search")...),
			Supplemental: "search: " + query,
			Rows:         1,
		}, true
	}
	return chrome.GuidanceSpec{
		Hints: append(chrome.RequiredGuidanceHints("a new note"),
			chrome.OptionalGuidanceHints(10, "p scope", "Esc close")...),
		Rows: 1,
	}, true
}

func (m *NotesViewModel) currentLocationGuidance() string {
	row := m.CurrentRow()
	if row == nil {
		return ""
	}
	label := strings.TrimSpace(row.OwnerLabel)
	if label == "" {
		label = strings.TrimSpace(m.LocationLabel(row.Location))
	}
	if label == "" {
		return ""
	}
	return "location: " + label
}
