package tui

import (
	"fmt"
)

// The outstanding-conflict indicator is workspace state, not an event: it is a
// line the status strip carries for as long as `.tt/conflicts/` holds a bundle,
// and it disappears once the CLI resolves the last one. The count itself is
// cached on the shell and injected as an accessor; see conflict_indicator_shell.go
// for where it is read.

// conflictIndicatorText is the strip's line for a given count. Zero renders
// nothing at all, so the strip keeps its height only while there is state to
// report.
func conflictIndicatorText(count int) string {
	switch {
	case count <= 0:
		return ""
	case count == 1:
		return "1 unresolved merge conflict — review with tt conflict list"
	default:
		return fmt.Sprintf("%d unresolved merge conflicts — review with tt conflict list", count)
	}
}

// setOutstandingConflicts injects the shell's conflict accessor. Like the note
// index it is a setter rather than a constructor parameter, because the Hub's
// preview painter builds an Outliner that must keep the nil accessor.
func (m *OutlinerModel) setOutstandingConflicts(accessor func() int) {
	m.outstandingConflicts = accessor
}

// conflictIndicator is the strip's line for the injected count.
func (m *OutlinerModel) conflictIndicator() string {
	if m == nil || m.outstandingConflicts == nil {
		return ""
	}
	return conflictIndicatorText(m.outstandingConflicts())
}
