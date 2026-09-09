package taskdetail

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// TestCommentCommitAdapterPreservesTransactionCorrelation guards the half of the
// shell's recovery feedback the panel owns: the shell can only install a
// correlated `u undo` toast if the panel's success message carries the
// transaction ID its coordinator commit returned. The shell-side half stays in
// internal/tui/recovery_feedback_test.go.
func TestCommentCommitAdapterPreservesTransactionCorrelation(t *testing.T) {
	transactionID := "subview-transaction"
	commit := func() tea.Msg {
		return coordinator.TransactionResultMsg{Result: app.TaskResult{TransactionID: transactionID}}
	}
	tests := []struct {
		name    string
		success func() tea.Msg
		id      func(tea.Msg) string
		owner   func(tea.Msg) (string, uint64)
	}{
		{
			name:    "comment",
			success: func() tea.Msg { return CommentAddedMsg{} },
			id:      func(msg tea.Msg) string { return msg.(CommentAddedMsg).TransactionID },
			owner:   func(msg tea.Msg) (string, uint64) { return msg.(CommentAddedMsg).Owner, msg.(CommentAddedMsg).Token },
		},
		{
			name:    "comment deleted",
			success: func() tea.Msg { return CommentDeletedMsg{} },
			id:      func(msg tea.Msg) string { return msg.(CommentDeletedMsg).TransactionID },
			owner: func(msg tea.Msg) (string, uint64) {
				return msg.(CommentDeletedMsg).Owner, msg.(CommentDeletedMsg).Token
			},
		},
		{
			name:    "comment edited",
			success: func() tea.Msg { return CommentEditedMsg{} },
			id:      func(msg tea.Msg) string { return msg.(CommentEditedMsg).TransactionID },
			owner:   func(msg tea.Msg) (string, uint64) { return msg.(CommentEditedMsg).Owner, msg.(CommentEditedMsg).Token },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			msg := adaptCommentCommit("task:p/t", 7, commit, test.success)()
			if got := test.id(msg); got != transactionID {
				t.Fatalf("transaction ID = %q, want %q", got, transactionID)
			}
			owner, token := test.owner(msg)
			if owner != "task:p/t" || token != 7 {
				t.Fatalf("correlation = (%q, %d), want (\"task:p/t\", 7)", owner, token)
			}
		})
	}
}

// TestCommentCommitAdapterCorrelatesRefusals holds the other half: a rejected
// transaction reaches the panel as its own message rather than the shared
// ErrorMsg, because only a correlated refusal can restore the right draft.
func TestCommentCommitAdapterCorrelatesRefusals(t *testing.T) {
	refused := errors.New("no such task")
	commit := func() tea.Msg { return coordinator.TransactionErrorMsg{Err: refused} }

	msg, ok := adaptCommentCommit("task:p/t", 9, commit, func() tea.Msg { return CommentAddedMsg{} })().(CommentMutationFailedMsg)
	if !ok {
		t.Fatal("a rejected comment transaction did not resolve to CommentMutationFailedMsg")
	}
	if !errors.Is(msg.Err, refused) {
		t.Fatalf("refusal error = %v, want %v", msg.Err, refused)
	}
	if msg.Owner != "task:p/t" || msg.Token != 9 {
		t.Fatalf("refusal correlation = (%q, %d)", msg.Owner, msg.Token)
	}
}
