package valuepicker

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/route"
)

// HandlePointer is the picker's share of the list surface's pointer answer, for
// an event the shell has already moved into this popup's own cells. A click on
// a row seats the cursor there, and in the sectioned date picker that is how a
// row of another section moves to it, since the cursor's row names the section
// as Tab would leave it. A double-click is Enter on the row and a right-click
// is Space where the picker has one; the wheel steps the cursor a row.
//
// The typed-entry modes and the rename workflow draw no list, so they swallow
// the pointer, and so does the search field. While the field is being edited,
// Enter and Space are its own keys, so a click only seats and the wheel only
// steps.
func (model *OperationValuePickerModel) HandlePointer(event route.PointerEvent) tea.Cmd {
	if model == nil || model.surface == nil || model.pointerListTop < 0 {
		return nil
	}
	key := model.surface.AnswerPointer(
		event.Offset(chrome.OverlayBoxContentLeft, model.pointerListTop),
		model.pointerMenu(),
	)
	if model.searching && (key == "enter" || key == "space") {
		return nil
	}
	return route.PressKey(key)
}

// pointerMenu reports whether Space is this picker's own act, which is when its
// guidance names it: a toggle on a multi-value picker, or the vocabulary sheet
// on the project-tag facet picker.
func (model *OperationValuePickerModel) pointerMenu() bool {
	return model.multi || model.provider == operation.ValueProviderFilterProjectTags
}

// PointerTarget is the row the picker's cursor names.
func (model *OperationValuePickerModel) PointerTarget() string {
	if model == nil {
		return ""
	}
	return model.surface.PointerTarget()
}

var (
	_ route.PointerView       = (*OperationValuePickerModel)(nil)
	_ route.PointerTargetView = (*OperationValuePickerModel)(nil)
)
