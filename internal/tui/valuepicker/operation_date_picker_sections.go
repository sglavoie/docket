package valuepicker

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/status"
)

// Reading which field a row writes and deciding whether that row may be
// committed are two different questions, and the combined picker needs them
// apart. A defer row a terminal task refuses is still a defer row: navigation
// must be able to land on it and the guidance must be able to name it, while
// `a` and Enter keep refusing it by its own stated reason. Keeping the field
// answer on the row's typed payload is also what keeps the active field honest
// — there is no second section index to drift out of step with the cursor.

type datePickerSection uint8

const (
	datePickerSectionDue datePickerSection = iota
	datePickerSectionDefer
	datePickerSectionCheckIn
)

var datePickerSections = []datePickerSection{
	datePickerSectionDue,
	datePickerSectionDefer,
	datePickerSectionCheckIn,
}

// dateRowSection names which scheduling section owns a row. Eligibility stays
// separate: a disabled defer row is still in Defer, and the editor-opening row
// is a Check-in action rather than a disguised deadline value.
func dateRowSection(item listsurface.ListItem) (datePickerSection, bool) {
	switch value := item.Data.(type) {
	case operation.OperationDateValue:
		if value.Deferred {
			return datePickerSectionDefer, true
		}
		return datePickerSectionDue, true
	case operation.OperationCheckInEditorAction:
		return datePickerSectionCheckIn, true
	default:
		return 0, false
	}
}

// dateSectionUnderCursor is the inspection reading: the section the cursor's
// row names, whether or not that row can be committed.
func (model *OperationValuePickerModel) dateSectionUnderCursor() (datePickerSection, bool) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return 0, false
	}
	return dateRowSection(item)
}

// typedDateSectionUnderCursor is the activation reading used by `a`. It keeps
// disabled-row refusal separate from section identity; Check-in has no typed
// single-date mode because its row opens the paired editor instead.
func (model *OperationValuePickerModel) typedDateSectionUnderCursor() (datePickerSection, error) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return 0, listsurface.ErrListItemNotFound
	}
	if !item.Enabled() {
		return 0, fmt.Errorf("%w: %s", listsurface.ErrListItemDisabled, item.DisabledReason)
	}
	section, ok := dateRowSection(item)
	if !ok {
		return 0, listsurface.ErrListItemNotFound
	}
	return section, nil
}

// sectionedDatePicker reports whether this is the combined scheduling picker.
// The due-only projection carries one field and no section headers, so there is
// nothing there to switch between.
func (model *OperationValuePickerModel) sectionedDatePicker() bool {
	return model != nil && model.provider == operation.ValueProviderDate
}

// switchDateSection moves the cursor to the first matching row in the adjacent
// visible section, skipping sections the query emptied. Tab moves forward and
// Shift+Tab backward. It lands on that section's first enabled match, so a
// Clear row disabled for an empty field is stepped over, and falls back to its
// first match when every row is disabled, which is how a section a terminal task
// refuses is still reached and its reason read. With only the current section
// matching, the cursor and query stay untouched.
func (model *OperationValuePickerModel) switchDateSection(direction int) tea.Msg {
	current, ok := model.dateSectionUnderCursor()
	if !ok {
		return status.StatusMsg("No matching date values")
	}
	currentIndex := 0
	for index, section := range datePickerSections {
		if section == current {
			currentIndex = index
			break
		}
	}
	for step := 1; step < len(datePickerSections); step++ {
		targetIndex := (currentIndex + direction*step + len(datePickerSections)) % len(datePickerSections)
		target := datePickerSections[targetIndex]
		first := listsurface.ListItemID("")
		for _, match := range model.surface.Matches() {
			section, ok := dateRowSection(match.Item)
			if !ok || section != target {
				continue
			}
			if match.Item.Enabled() {
				model.surface.SetCursorToID(match.Item.ID)
				return nil
			}
			if first == "" {
				first = match.Item.ID
			}
		}
		if first != "" {
			model.surface.SetCursorToID(first)
			return nil
		}
	}
	return nil
}

// dateEntryNoun names the date field the cursor sits in, so the one `a` binding
// says which value it would write. Check-in returns no noun because its row
// opens the paired editor; the due-only picker keeps the plain noun.
func (model *OperationValuePickerModel) dateEntryNoun() string {
	if !model.sectionedDatePicker() {
		return "date"
	}
	section, ok := model.dateSectionUnderCursor()
	if !ok {
		return ""
	}
	switch section {
	case datePickerSectionDue:
		return "due date"
	case datePickerSectionDefer:
		return "defer date"
	default:
		return ""
	}
}
