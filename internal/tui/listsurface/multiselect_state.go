package listsurface

import (
	"fmt"
	"sort"

	"github.com/sglavoie/tt/internal/tui/taskref"
)

// MultiSelectState provides reusable multi-selection state for views that
// support selecting multiple tasks. Embed this in view models to share the
// common fields and basic selection operations.
type MultiSelectState struct {
	MultiSelectMode      bool
	Selected             map[taskref.TaskIdentity]bool
	rangeSelectionActive bool
	RangeAnchor          taskref.TaskIdentity
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
	s.Selected = make(map[taskref.TaskIdentity]bool)
	s.FreezeRangeSelection()
}

func (s *MultiSelectState) SetSelection(ids []taskref.TaskIdentity) {
	s.Selected = make(map[taskref.TaskIdentity]bool, len(ids))
	for _, id := range ids {
		if id.Valid() {
			s.Selected[id] = true
		}
	}
}

// SelectedCount returns the number of currently selected items.
func (s *MultiSelectState) SelectedCount() int {
	return len(s.Selected)
}

// SelectedTaskIdentities returns a deterministic defensive copy suitable for
// operation target resolution.
func (s *MultiSelectState) SelectedTaskIdentities() []taskref.TaskIdentity {
	selected := make([]taskref.TaskIdentity, 0, len(s.Selected))
	for identity, active := range s.Selected {
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
	return s.MultiSelectMode
}

// IsRangeSelectionActive reports whether cursor movement should continue to
// recalculate the inclusive visible-task range.
func (s *MultiSelectState) IsRangeSelectionActive() bool {
	return s.rangeSelectionActive
}

// StartRangeSelection replaces every prior selection with the anchor task.
func (s *MultiSelectState) StartRangeSelection(anchor taskref.TaskIdentity) {
	if !anchor.Valid() {
		return
	}
	s.MultiSelectMode = true
	s.rangeSelectionActive = true
	s.RangeAnchor = anchor
	s.SetSelection([]taskref.TaskIdentity{anchor})
}

// FreezeRangeSelection leaves the selected identities intact while stopping
// cursor-driven range recalculation.
func (s *MultiSelectState) FreezeRangeSelection() {
	s.rangeSelectionActive = false
	s.RangeAnchor = taskref.TaskIdentity{}
}

// ReplaceSelection installs a frozen visual selection.
func (s *MultiSelectState) ReplaceSelection(ids []taskref.TaskIdentity) {
	s.MultiSelectMode = true
	s.FreezeRangeSelection()
	s.SetSelection(ids)
}

// ClassifySelection compares the persistent identities with the current
// visible projection. Duplicate visible rows count once.
func (s *MultiSelectState) ClassifySelection(visible []taskref.TaskIdentity) SelectionCounts {
	counts := SelectionCounts{Total: len(s.SelectedTaskIdentities())}
	seen := make(map[taskref.TaskIdentity]bool, len(visible))
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
	s.MultiSelectMode = false
	s.ClearSelection()
}

func (s *MultiSelectState) ToggleTaskIdentity(id taskref.TaskIdentity) {
	if !id.Valid() {
		return
	}
	if s.Selected[id] {
		delete(s.Selected, id)
		return
	}
	if s.Selected == nil {
		s.Selected = make(map[taskref.TaskIdentity]bool)
	}
	s.Selected[id] = true
}

func (s *MultiSelectState) SelectTaskIdentity(id taskref.TaskIdentity) {
	if !id.Valid() {
		return
	}
	if s.Selected == nil {
		s.Selected = make(map[taskref.TaskIdentity]bool)
	}
	s.Selected[id] = true
}

func (s *MultiSelectState) DeselectTaskIdentity(id taskref.TaskIdentity) {
	delete(s.Selected, id)
}

func (s *MultiSelectState) IsTaskIdentitySelected(id taskref.TaskIdentity) bool {
	return id.Valid() && s.Selected[id]
}

// CheckboxTaskIdentity renders an identity-safe multi-select marker for
// cross-project task lists.
func (s *MultiSelectState) CheckboxTaskIdentity(id taskref.TaskIdentity) string {
	if !s.MultiSelectMode {
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
	s.MultiSelectMode = !s.MultiSelectMode
	if !s.MultiSelectMode {
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
