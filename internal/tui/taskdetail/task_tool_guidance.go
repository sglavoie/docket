package taskdetail

import (
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/keymap"
)

// discardConfirmAction names the questions these panels raise over unsaved text
// rather than over stored work. They take the editor's own chrome, since
// declining one keeps the draft and stays in the composer.
func discardConfirmAction(action string) bool {
	return action == "discard-add" || action == "discard-edit"
}

// taskToolGuidanceRows is the budget both panels publish at in every sub-state.
// The footer is the shell's one block now, so the row the panel's own divider
// used to cost is the row a narrow terminal wraps onto: at the floor the browse
// keys keep the `Space` menu, which is the only route to Delete, beside the exit
// and `? help`. A width that fits them on one row still draws one.
const taskToolGuidanceRows = 2

func taskToolGuidanceSpec(confirm *chrome.ConfirmDialog, km *keymap.KeyMap) (chrome.GuidanceSpec, bool) {
	if confirm.IsActive() {
		if discardConfirmAction(confirm.Action()) {
			return chrome.DiscardGuidanceSpec(), true
		}
		return chrome.ConfirmationGuidanceSpec(), true
	}
	return chrome.GuidanceSpecFromKeyMap(km, taskToolGuidanceRows)
}

func (m *CommentsModel) guidanceSpec() (chrome.GuidanceSpec, bool) {
	return taskToolGuidanceSpec(&m.Confirm, m.BuildKeyMap())
}

func (m *TaskAttachmentsModel) guidanceSpec() (chrome.GuidanceSpec, bool) {
	return taskToolGuidanceSpec(&m.Confirm, m.BuildKeyMap())
}

// ContextualGuidance answers route.ContextualGuidanceProvider: the panel hands
// its keys to the shell footer and draws none of its own, so the frame carries
// one divider and the hints sit on the terminal's last row.
func (m *CommentsModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	return m.guidanceSpec()
}

// ContextualGuidance answers route.ContextualGuidanceProvider on the same terms
// as the comments panel.
func (m *TaskAttachmentsModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	return m.guidanceSpec()
}

// taskToolAvailableRows is what the panel's fixed rows leave for its list. The
// footer is the shell's and is no term here: the height handed in is already
// the frame less whatever that footer costs beyond its baseline.
func taskToolAvailableRows(fixed string, height int) int {
	if height <= 0 {
		return 1 << 20
	}
	return max(height-composedRows(fixed), 0)
}

func taskToolMaxVisible(total, availableRows, maxRowsPerItem int) int {
	if total <= 0 || availableRows <= 0 {
		return 0
	}
	maxRowsPerItem = max(maxRowsPerItem, 1)
	visible := min(total, availableRows/maxRowsPerItem)
	if visible < total {
		visible = max((availableRows-1)/maxRowsPerItem, 0)
	}
	return visible
}
