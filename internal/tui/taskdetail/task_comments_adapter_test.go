package taskdetail

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// TestSubviewAdapterPreservesTransactionCorrelation guards the half of the
// shell's recovery feedback the panel owns: the shell can only install a
// correlated `u undo` toast if the panel's success message carries the
// transaction ID its coordinator commit returned. The shell-side half stays in
// internal/tui/recovery_feedback_test.go.
func TestSubviewAdapterPreservesTransactionCorrelation(t *testing.T) {
	transactionID := "subview-transaction"
	commit := func() tea.Msg {
		return coordinator.TransactionResultMsg{Result: app.TaskResult{TransactionID: transactionID}}
	}
	tests := []struct {
		name    string
		success func() tea.Msg
		id      func(tea.Msg) string
	}{
		{name: "comment", success: func() tea.Msg { return CommentAddedMsg{} }, id: func(msg tea.Msg) string { return msg.(CommentAddedMsg).TransactionID }},
		{name: "comment edited", success: func() tea.Msg { return CommentEditedMsg{} }, id: func(msg tea.Msg) string { return msg.(CommentEditedMsg).TransactionID }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.id(adaptSubviewCommit(commit, test.success)()); got != transactionID {
				t.Fatalf("transaction ID = %q, want %q", got, transactionID)
			}
		})
	}
}
