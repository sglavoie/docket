package valuepicker

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// Reading which field a row writes and deciding whether that row may be
// committed are two different questions, and the combined picker needs them
// apart. A defer row a terminal task refuses is still a defer row: navigation
// must be able to land on it and the guidance must be able to name it, while
// `a` and Enter keep refusing it by its own stated reason. Keeping the field
// answer on the row's typed payload is also what keeps the active field honest
// — there is no second section index to drift out of step with the cursor.

// dateRowField names which of the two fields one row writes. It answers for a
// disabled row as readily as an enabled one.
func dateRowField(item listsurface.ListItem) (deferred bool, ok bool) {
	value, ok := item.Data.(operation.OperationDateValue)
	if !ok {
		return false, false
	}
	return value.Deferred, true
}

// dateFieldUnderCursor is the inspection reading: the field the cursor's row
// names, whether or not that row can be committed.
func (model *OperationValuePickerModel) dateFieldUnderCursor() (bool, bool) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return false, false
	}
	return dateRowField(item)
}

// dateSectionUnderCursor is the activation reading: the same field, refused
// when the row is disabled so the inline mode cannot type past a refusal the
// preset rows already carry.
func (model *OperationValuePickerModel) dateSectionUnderCursor() (bool, error) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return false, listsurface.ErrListItemNotFound
	}
	if !item.Enabled() {
		return false, fmt.Errorf("%w: %s", listsurface.ErrListItemDisabled, item.DisabledReason)
	}
	deferred, ok := dateRowField(item)
	if !ok {
		return false, listsurface.ErrListItemNotFound
	}
	return deferred, nil
}

// sectionedDatePicker reports whether this picker holds both fields. The
// due-only projection carries one field and no section headers, so there is
// nothing there to switch between.
func (model *OperationValuePickerModel) sectionedDatePicker() bool {
	return model != nil && model.provider == operation.ValueProviderDate
}

// switchDateSection moves the cursor to the first matching row of the other
// field. It reads the matches rather than the items, so a switch under an
// active search lands on a row that is actually on screen, and it accepts a
// disabled row on purpose: crossing into a refused defer section is how the
// refusal gets read. Nothing is selected and the query is untouched, so a
// switch that finds nowhere to go leaves the picker exactly as it was and says
// which section it could not reach.
func (model *OperationValuePickerModel) switchDateSection() tea.Msg {
	current, ok := model.dateFieldUnderCursor()
	if !ok {
		return status.StatusMsg("No date values match")
	}
	for _, match := range model.surface.Matches() {
		deferred, ok := dateRowField(match.Item)
		if !ok || deferred == current {
			continue
		}
		model.surface.SetCursorToID(match.Item.ID)
		return nil
	}
	return status.StatusMsg(fmt.Sprintf("No matching %s values", DateSectionLabel(!current)))
}

// dateEntryNoun names the field the cursor sits in, so the one `a` binding says
// which of the two fields a typed value would write. The due-only picker has
// one field and no sections, so it keeps the plain noun.
func (model *OperationValuePickerModel) dateEntryNoun() string {
	if !model.sectionedDatePicker() {
		return "date"
	}
	deferred, ok := model.dateFieldUnderCursor()
	if !ok {
		return "date"
	}
	if deferred {
		return "defer date"
	}
	return "due date"
}
