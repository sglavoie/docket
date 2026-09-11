package valuepicker

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// A preset key is an immediate answer, not a navigation aid: it applies one row
// and closes the picker in a single press. What it may never do is apply a row
// other than the one it names, so it resolves through the field the cursor's own
// row declares and through the stable preset identity, and then looks that
// identity up among the matches currently on screen. Everything after that is
// Enter's path: SelectedValue does the validating, the frozen request travels
// untouched, and the executor sees a selection it cannot tell from a chosen one.
func (model *OperationValuePickerModel) handleDatePresetShortcut(key string) (tea.Msg, bool, bool) {
	if !chrome.DatePresetShortcutKey(key) {
		return nil, false, false
	}
	deferred, ok := model.dateFieldUnderCursor()
	if !ok {
		// No row under the cursor means no field, and a shortcut with no field to
		// resolve in has nothing to apply. The key is still claimed, because it is
		// this picker's key whether or not a search left it a row to answer with.
		return status.StatusMsg("No date values match"), false, true
	}
	presetID, ok := chrome.DatePresetIDForKey(deferred, key)
	if !ok {
		// The key belongs to the other field. Silence is the whole answer: `t` is
		// Today and must never become a deferral, and an hour key must never become
		// a deadline, so neither field ever borrows the other's row.
		return nil, false, true
	}
	if !model.surface.SetCursorToID(DateListItemID(deferred, presetID)) {
		// SetCursorToID searches the matches alone and leaves the cursor where it
		// was on a miss, so a filtered-out preset says so and changes nothing: not
		// the query that hid it, and above all not some other row.
		return status.StatusMsg(fmt.Sprintf(
			"%s is filtered out", datePresetLabel(deferred, presetID),
		)), false, true
	}
	value, err := model.SelectedValue()
	if err != nil {
		// A disabled row keeps its own stated reason, the same one `a` and Enter
		// report. The cursor stays on it, which is how that reason gets read.
		return status.StatusMsg(err.Error()), false, true
	}
	return operation.OperationValueSelectedMsg{Request: model.request, Value: value}, true, true
}

// datePresetLabel names a preset the way its row does, so a refusal speaks the
// list's language rather than the table's internal identity.
func datePresetLabel(deferred bool, presetID string) string {
	for _, preset := range OperationDatePresets(deferred, time.Now()) {
		if preset.id == presetID {
			return preset.label
		}
	}
	return presetID
}
