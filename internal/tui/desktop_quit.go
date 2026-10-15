package tui

import "github.com/sglavoie/tt/internal/tui/presentation"

// quitContentOwner reports actual unsaved content, independently of transient
// navigation. Inspect the entire live presentation path: Help or a picker may
// cover an editor without making its draft safe to discard.
type quitContentOwner interface{ QuitBlockReason() string }

func (m *AppModel) quitBlockReason() string {
	if m.checkInSetup != nil && m.checkInSetup.busy {
		return "A check-in change is still being saved."
	}
	if p := m.operations.TextPrompt; p != nil {
		if reason := p.QuitBlockReason(); reason != "" {
			return reason
		}
	}
	if p := m.operations.CheckInEditor; p != nil {
		if reason := p.QuitBlockReason(); reason != "" {
			return reason
		}
	}
	if p := m.operations.ValuePicker; p != nil {
		if reason := p.QuitBlockReason(); reason != "" {
			return reason
		}
	}
	path := m.presentationPath()
	for i := len(path) - 1; i >= 0; i-- {
		var owner quitContentOwner
		if path[i] == presentation.ViewTaskComposer && m.taskComposer != nil {
			owner = m.taskComposer
		} else if m.viewNav.views != nil {
			owner, _ = m.viewNav.views.Get(path[i]).(quitContentOwner)
		}
		if owner != nil {
			if reason := owner.QuitBlockReason(); reason != "" {
				return reason
			}
		}
	}
	return ""
}
