package tui

import (
	"fmt"
	"strings"
)

// ContextualGuidance returns the controls for the Notes interaction that owns
// input right now. Modal commit/cancel controls are required and receive the
// two-row interaction budget; ordinary browse and causal empty states stay on
// one row.
func (m *NotesViewModel) ContextualGuidance() (GuidanceSpec, bool) {
	if m.locationPicker != nil {
		return m.locationPicker.guidanceSpec()
	}

	switch {
	case m.confirmDiscard:
		return GuidanceSpec{
			Hints: requiredGuidanceHints("Y discard", "N/Esc keep editing"),
			Rows:  2,
		}, true
	case m.confirm.IsActive():
		return GuidanceSpec{
			Hints: requiredGuidanceHints("Y delete", "N/Esc cancel"),
			Rows:  2,
		}, true
	case m.renaming:
		return GuidanceSpec{
			Hints:        requiredGuidanceHints("Enter save", "Esc cancel"),
			Supplemental: m.currentLocationGuidance(),
			Rows:         2,
		}, true
	case m.searchEditing:
		supplemental := ""
		if len(m.visibleRows) == 0 {
			supplemental = "no matches"
		}
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Enter apply search", "Esc cancel edit"),
				optionalGuidanceHints(10, "Ctrl+J/K navigate")...),
			Supplemental: supplemental,
			Rows:         2,
		}, true
	}

	if len(m.visibleRows) == 0 {
		return m.emptyGuidance()
	}

	spec, ok := guidanceSpecFromKeyMap(m.buildKeyMap(), 1)
	if !ok {
		return GuidanceSpec{}, false
	}
	if query := strings.TrimSpace(m.searchApplied); query != "" {
		spec.Hints = append(requiredGuidanceHints("Esc clear search"), spec.Hints...)
		spec.Supplemental = fmt.Sprintf("search: %s", query)
	}
	spec.Supplemental = joinGuidanceContext(spec.Supplemental, m.currentLocationGuidance())
	return spec, true
}

func (m *NotesViewModel) emptyGuidance() (GuidanceSpec, bool) {
	if m.snapshotError != nil {
		return GuidanceSpec{
			Hints: requiredGuidanceHints("Esc close"),
			Rows:  1,
		}, true
	}
	if query := strings.TrimSpace(m.searchApplied); query != "" {
		return GuidanceSpec{
			Hints: append(requiredGuidanceHints("Esc clear search"),
				optionalGuidanceHints(10, "/ edit search")...),
			Supplemental: "search: " + query,
			Rows:         1,
		}, true
	}
	return GuidanceSpec{
		Hints: append(requiredGuidanceHints("a choose location"),
			optionalGuidanceHints(10, "p scope", "Esc close")...),
		Rows: 1,
	}, true
}

func (m *NotesViewModel) currentLocationGuidance() string {
	row := m.currentRow()
	if row == nil {
		return ""
	}
	label := strings.TrimSpace(row.OwnerLabel)
	if label == "" {
		label = strings.TrimSpace(m.locationLabel(row.Location))
	}
	if label == "" {
		return ""
	}
	return "location: " + label
}

func joinGuidanceContext(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return strings.Join(values, " · ")
}
