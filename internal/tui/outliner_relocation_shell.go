package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The shell half of the relocation capture. outliner_relocation.go owns the
// capture itself and every cursor decision made against it; this file owns the
// two seams the shell needs — resolving a capture from an operation's targets,
// and carrying one alongside a transaction result until the committed rows are
// visible.

func (m *AppModel) captureOutlinerRelocationForTargets(targets ResolvedOperationTargets) *outlinerRelocationCapture {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
	}
	identities := make([]taskref.TaskIdentity, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.Task.Valid() {
			identities = append(identities, target.Task)
		}
	}
	return out.captureOutlinerRelocation(identities)
}

// withOutlinerRelocationCapture decorates only successful batch results. Error
// messages pass through unchanged, so failed mutations cannot pollute history.
func withOutlinerRelocationCapture(cmd tea.Cmd, capture *outlinerRelocationCapture) tea.Cmd {
	if cmd == nil || capture == nil {
		return cmd
	}
	return func() tea.Msg {
		msg := cmd()
		switch result := msg.(type) {
		case BatchTransactionResultMsg:
			copy := *capture
			copy.TransactionID = result.TransactionID
			result.outlinerRelocation = &copy
			return result
		case TransactionResultMsg:
			copy := *capture
			copy.TransactionID = result.Result.TransactionID
			result.outlinerRelocation = &copy
			return result
		default:
			return msg
		}
	}
}

func (m *AppModel) finishSynchronousOutlinerRelocation(cmd tea.Cmd, capture *outlinerRelocationCapture, successMessage, transactionID string) tea.Cmd {
	recorded := false
	var jumpCmd tea.Cmd
	if m != nil && m.crossOutliner != nil && capture != nil {
		jumpCmd, recorded = m.crossOutliner.finishOutlinerRelocation(capture, true)
	}
	feedbackCmd := m.setRecoveryFeedback(successMessage, transactionID, recorded)
	return tea.Batch(cmd, jumpCmd, feedbackCmd)
}
