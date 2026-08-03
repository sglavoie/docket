package tui

import (
	"fmt"
	"sort"
)

// MultiSelectState provides reusable multi-selection state for views that
// support selecting multiple tasks. Embed this in view models to share the
// common fields and basic selection operations.
type MultiSelectState struct {
	multiSelectMode        bool
	selectedTaskIdentities map[TaskIdentity]bool
	rangeSelectionActive   bool
	rangeAnchor            TaskIdentity
}

// SelectionCounts classifies a persistent selection against the task rows in
// the current projection. Total includes every selected project-qualified
// identity; Hidden is the part not represented by a visible task row.
type SelectionCounts struct {
	Total   int
	Visible int
	Hidden  int
}

// ClearSelection resets all selections without changing multiSelectMode.
func (s *MultiSelectState) ClearSelection() {
	s.selectedTaskIdentities = make(map[TaskIdentity]bool)
	s.FreezeRangeSelection()
}

func (s *MultiSelectState) replaceSelection(ids []TaskIdentity) {
	s.selectedTaskIdentities = make(map[TaskIdentity]bool, len(ids))
	for _, id := range ids {
		if id.Valid() {
			s.selectedTaskIdentities[id] = true
		}
	}
}

// SelectedCount returns the number of currently selected items.
func (s *MultiSelectState) SelectedCount() int {
	return len(s.selectedTaskIdentities)
}

// SelectedTaskIdentities returns a deterministic defensive copy suitable for
// operation target resolution.
func (s *MultiSelectState) SelectedTaskIdentities() []TaskIdentity {
	selected := make([]TaskIdentity, 0, len(s.selectedTaskIdentities))
	for identity, active := range s.selectedTaskIdentities {
		if active && identity.Valid() {
			selected = append(selected, identity)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].ProjectID != selected[j].ProjectID {
			return selected[i].ProjectID < selected[j].ProjectID
		}
		return selected[i].TaskID < selected[j].TaskID
	})
	return selected
}

// IsMultiSelectActive reports whether multi-select mode is on. Used by the
// global key dispatcher to keep space bound to toggle-select instead of peek.
func (s *MultiSelectState) IsMultiSelectActive() bool {
	return s.multiSelectMode
}

// IsRangeSelectionActive reports whether cursor movement should continue to
// recalculate the inclusive visible-task range.
func (s *MultiSelectState) IsRangeSelectionActive() bool {
	return s.rangeSelectionActive
}

// StartRangeSelection replaces every prior selection with the anchor task.
func (s *MultiSelectState) StartRangeSelection(anchor TaskIdentity) {
	if !anchor.Valid() {
		return
	}
	s.multiSelectMode = true
	s.rangeSelectionActive = true
	s.rangeAnchor = anchor
	s.replaceSelection([]TaskIdentity{anchor})
}

// FreezeRangeSelection leaves the selected identities intact while stopping
// cursor-driven range recalculation.
func (s *MultiSelectState) FreezeRangeSelection() {
	s.rangeSelectionActive = false
	s.rangeAnchor = TaskIdentity{}
}

// ReplaceSelection installs a frozen visual selection.
func (s *MultiSelectState) ReplaceSelection(ids []TaskIdentity) {
	s.multiSelectMode = true
	s.FreezeRangeSelection()
	s.replaceSelection(ids)
}

// ClassifySelection compares the persistent identities with the current
// visible projection. Duplicate visible rows count once.
func (s *MultiSelectState) ClassifySelection(visible []TaskIdentity) SelectionCounts {
	counts := SelectionCounts{Total: len(s.SelectedTaskIdentities())}
	seen := make(map[TaskIdentity]bool, len(visible))
	for _, identity := range visible {
		if !identity.Valid() || seen[identity] {
			continue
		}
		seen[identity] = true
		if s.IsTaskIdentitySelected(identity) {
			counts.Visible++
		}
	}
	counts.Hidden = counts.Total - counts.Visible
	return counts
}

// ExitMultiSelect deactivates multi-select mode and clears all selections.
func (s *MultiSelectState) ExitMultiSelect() {
	s.multiSelectMode = false
	s.ClearSelection()
}

func (s *MultiSelectState) ToggleTaskIdentity(id TaskIdentity) {
	if !id.Valid() {
		return
	}
	if s.selectedTaskIdentities[id] {
		delete(s.selectedTaskIdentities, id)
		return
	}
	if s.selectedTaskIdentities == nil {
		s.selectedTaskIdentities = make(map[TaskIdentity]bool)
	}
	s.selectedTaskIdentities[id] = true
}

func (s *MultiSelectState) SelectTaskIdentity(id TaskIdentity) {
	if !id.Valid() {
		return
	}
	if s.selectedTaskIdentities == nil {
		s.selectedTaskIdentities = make(map[TaskIdentity]bool)
	}
	s.selectedTaskIdentities[id] = true
}

func (s *MultiSelectState) DeselectTaskIdentity(id TaskIdentity) {
	delete(s.selectedTaskIdentities, id)
}

func (s *MultiSelectState) IsTaskIdentitySelected(id TaskIdentity) bool {
	return id.Valid() && s.selectedTaskIdentities[id]
}

// CheckboxTaskIdentity renders an identity-safe multi-select marker for
// cross-project task lists.
func (s *MultiSelectState) CheckboxTaskIdentity(id TaskIdentity) string {
	if !s.multiSelectMode {
		return ""
	}
	if s.IsTaskIdentitySelected(id) {
		return "[x] "
	}
	return "[ ] "
}

// ToggleMode enters or exits multi-select mode. On entry, statusMsg is set to
// enterMsg. On exit, selections are cleared and statusMsg is set to empty.
func (s *MultiSelectState) ToggleMode(statusMsg *string, enterMsg string) {
	s.multiSelectMode = !s.multiSelectMode
	if !s.multiSelectMode {
		s.ClearSelection()
		*statusMsg = ""
	} else {
		*statusMsg = enterMsg
	}
}

// UpdateStatus sets the status message based on current selection count.
// noSelMsg is used when count is 0; withSelFmt (with one %d) when count > 0.
func (s *MultiSelectState) UpdateStatus(statusMsg *string, noSelMsg, withSelFmt string) {
	count := s.SelectedCount()
	if count == 0 {
		*statusMsg = noSelMsg
	} else {
		*statusMsg = fmt.Sprintf(withSelFmt, count)
	}
}

// FinishBulk records a successful non-delete bulk result without changing the
// persistent selection. Delete owns its explicit post-commit clear path.
func (s *MultiSelectState) FinishBulk(statusMsg *string, msg string) {
	s.FreezeRangeSelection()
	*statusMsg = msg
}
