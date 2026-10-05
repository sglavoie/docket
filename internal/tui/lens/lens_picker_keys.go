package lens

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// HandleKey implements the shell's compact key-interceptor contract.
//
//nolint:gocyclo // Key dispatch stays centralized so every picker state has one unwind path.
func (model *NamedLensPickerModel) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.Surface == nil {
		return nil, true, NamedLensPickerClosedMsg{}
	}
	key := msg.String()
	itemBefore, _ := model.Surface.CurrentItem()
	defer func() {
		itemAfter, _ := model.Surface.CurrentItem()
		if itemBefore.ID != itemAfter.ID {
			model.detailOffset = 0
		}
	}()
	// Detail paging belongs to the preview even while the search field is open.
	if key == "ctrl+f" || key == "ctrl+b" {
		delta := max(model.detailPageRows-1, 1)
		if key == "ctrl+b" {
			delta = -delta
		}
		model.detailOffset = min(max(min(model.detailOffset, model.detailMaxOffset)+delta, 0), model.detailMaxOffset)
		return nil, false, nil
	}
	if model.Searching {
		switch key {
		case "esc":
			// One Esc clears a typed query; the next leaves search. Closing the
			// picker is the third, and belongs to browse mode alone.
			if model.Query != "" {
				model.setQuery("")
				return nil, false, nil
			}
			model.Searching = false
			return nil, false, nil
		case "enter":
			model.Searching = false
			return nil, false, nil
		case "up":
			model.Surface.MoveCursor(-1)
			return nil, false, nil
		case "down":
			model.Surface.MoveCursor(1)
			return nil, false, nil
		default:
			model.setQuery(model.searchInput.HandleKey(model.Query, msg))
			return nil, false, nil
		}
	}

	switch key {
	case "esc":
		return nil, true, NamedLensPickerClosedMsg{}
	case "/":
		model.Searching = true
	case "up", "k", "ctrl+p":
		model.Surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.Surface.MoveCursor(1)
	case "home", "g":
		model.Surface.MoveCursorToStart()
	case "end", "G":
		model.Surface.MoveCursorToEnd()
	case "pgup":
		model.Surface.PageCursor(-1)
	case "pgdown":
		model.Surface.PageCursor(1)
	case "enter":
		return model.activateCursor()
	case "space":
		return nil, false, model.openActionsForCursor()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Spelled out rather than reached through a default arm: a default would
		// silently swallow every key this switch has not learned yet, and the search
		// branch above already claims digits as ordinary characters while typing.
		return model.applyOrdinal(int(key[0] - '0'))
	}
	return nil, false, nil
}

// applyOrdinal is the quick-apply digit. It moves the cursor onto the row it
// names before dispatching, so a refused apply leaves the picker on the row that
// raised the reason, exactly as Enter would have. Out of range refuses in the
// action rows' own vocabulary and moves nothing.
func (model *NamedLensPickerModel) applyOrdinal(ordinal int) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.Surface == nil {
		return nil, false, nil
	}
	if ordinal < 1 || ordinal > len(model.Ordinals) {
		model.notice = lensPickerOrdinalRefusal(len(model.Ordinals))
		return nil, false, status.StatusMsg(model.notice)
	}
	if !model.SelectLens(model.Ordinals[ordinal-1]) {
		model.notice = lensPickerOrdinalRefusal(len(model.Ordinals))
		return nil, false, status.StatusMsg(model.notice)
	}
	return model.activateCursor()
}

// lensPickerOrdinalRefusal names the numbering that actually exists, so the
// refusal doubles as the legend for the next press. It counts what is numbered
// rather than what is listed, which is the honest wording once a tenth saved
// lens is on screen carrying no digit.
func lensPickerOrdinalRefusal(numbered int) string {
	switch {
	case numbered <= 0:
		return LensPickerOrdinalEmptyRefusal
	case numbered == 1:
		return "Only 1 lens is numbered — press 1"
	default:
		return fmt.Sprintf("Only %d lenses are numbered — press 1-%d", numbered, numbered)
	}
}

// activateCursor is Enter: Save opens the name prompt and Add built-in view
// opens the gallery, both through the executor's own value step, and a lens row
// applies and closes. Neither action row names a target, because the record each
// one writes does not exist yet.
func (model *NamedLensPickerModel) activateCursor() (tea.Cmd, bool, tea.Msg) {
	item, ok := model.Surface.CurrentItem()
	if !ok {
		return nil, false, nil
	}
	if action, isAction := lensPickerActionOperation(item.ID); isAction {
		result, dismissed := model.Execute(model.request(action, operation.EntityIdentity{}))
		return nil, dismissed, NamedLensPickerResultMsg{Result: result}
	}
	lens, ok := item.Data.(config.NamedLens)
	if !ok {
		model.notice = "That row is not a saved lens"
		return nil, false, status.StatusMsg(model.notice)
	}
	result, dismissed := model.Execute(model.request(OperationLensApply, operation.LensEntity(lens.ID)))
	return nil, dismissed, NamedLensPickerResultMsg{Result: result}
}

// lensPickerActionOperation maps one query-exempt row onto the operation Enter
// dispatches from it. A saved-lens row answers false, which is what keeps the
// two act rows and the population rows on separate dispatch paths.
func lensPickerActionOperation(id listsurface.ListItemID) (operation.OperationID, bool) {
	switch id {
	case lensPickerCreateItemID:
		return OperationLensCreate, true
	case LensPickerAddTemplateItemID:
		return OperationLensAddTemplate, true
	default:
		return "", false
	}
}

// openActionsForCursor is Space: the management sheet nests above the picker
// rather than replacing it, so canceling anything inside it lands back here.
func (model *NamedLensPickerModel) openActionsForCursor() tea.Msg {
	item, ok := model.Surface.CurrentItem()
	if !ok {
		return nil
	}
	lens, ok := item.Data.(config.NamedLens)
	if !ok {
		// Each action row writes a record that does not exist yet, so neither names
		// a target and neither has anything to rename, replace, or delete. The
		// notice names that row's own act rather than a shared refusal, so Space is
		// still a hint about where Enter goes.
		model.notice = LensPickerCreateSpaceNotice
		if item.ID == LensPickerAddTemplateItemID {
			model.notice = LensPickerAddTemplateSpaceHint
		}
		return status.StatusMsg(model.notice)
	}
	model.notice = ""
	return NamedLensActionsRequestMsg{
		Entity: operation.LensEntity(lens.ID),
		Name:   lens.Name,
	}
}
