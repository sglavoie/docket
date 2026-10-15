package valuepicker

import (
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/tui/chrome"
)

func (model *OperationValuePickerModel) QuitBlockReason() string {
	if model.tagAdder.AddTagMode && strings.TrimSpace(model.tagAdder.AddTagInput.Value()) != "" ||
		model.dateAdder.AddDateMode && strings.TrimSpace(model.dateAdder.AddDateInput.Value()) != "" ||
		model.rename.stage == vocabularyRenameNaming && model.rename.input.Value() != model.rename.from ||
		model.rename.stage == vocabularyRenameConfirming {
		return "Your picker entry has not been applied."
	}
	if model.membershipInitial != nil {
		for id, current := range model.membershipCurrent {
			if current != model.membershipInitial[id] {
				return "Your selection changes have not been applied."
			}
		}
		if id, _, ok := model.selectedContextRow(); ok && id != model.ContextInitial {
			return "Your context selection has not been applied."
		}
	} else if model.multi {
		initial := slices.Clone(model.initialSelection)
		selected := model.surface.SelectedIDs()
		slices.Sort(initial)
		slices.Sort(selected)
		if !slices.Equal(initial, selected) {
			return "Your selection changes have not been applied."
		}
	}
	return ""
}

func (model *OperationValuePickerModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if model.tagAdder.AddTagMode {
		return chrome.DesktopInput(&model.tagAdder.AddTagInput, nil)
	}
	if model.dateAdder.AddDateMode {
		return chrome.DesktopInput(&model.dateAdder.AddDateInput, nil)
	}
	if model.rename.stage == vocabularyRenameNaming {
		return chrome.DesktopInput(&model.rename.input, func() { model.rename.failure = "" })
	}
	return nil
}
