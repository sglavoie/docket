package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newRecoveryFeedbackEnv(t *testing.T) (*AppModel, string) {
	t.Helper()
	task := seedTask(t, 1, "Recovery target", models.StateBacklog)
	dataDir := t.TempDir()
	project := newLedgerUndoProject(t, dataDir, "recovery-feedback", task)
	model := newLedgerUndoApp(t, dataDir, project, task.ID, true)
	title := "Recovered title"
	result, err := model.appService.Edit(app.EditRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:   &title,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if result.TransactionID == "" {
		t.Fatal("application result omitted transaction ID")
	}
	return model, result.TransactionID
}

func TestRecoveryFeedbackRequiresExactNewestTransactionID(t *testing.T) {
	t.Run("matching transaction installs undo", func(t *testing.T) {
		model, transactionID := newRecoveryFeedbackEnv(t)
		cmd := model.setRecoveryFeedback("Updated task", transactionID, false)
		if cmd == nil || model.statusUndoCb == nil {
			t.Fatal("matching newest transaction did not install recovery callback")
		}
		if model.statusMessage != "Edit Task · u undo" {
			t.Fatalf("status = %q, want canonical recovery feedback", model.statusMessage)
		}
		if model.statusTransactionID != transactionID || !time.Now().Before(model.statusExpiresAt) {
			t.Fatal("matching recovery feedback did not retain its ID and six-second window")
		}
	})

	t.Run("missing transaction degrades", func(t *testing.T) {
		model, _ := newRecoveryFeedbackEnv(t)
		if cmd := model.setRecoveryFeedback("Updated task", "", false); cmd == nil {
			t.Fatal("plain success did not receive its normal timeout")
		}
		if model.statusMessage != "Updated task" || model.statusUndoCb != nil || !model.statusExpiresAt.IsZero() {
			t.Fatalf("missing-ID feedback did not degrade safely: status=%q callback=%v expiry=%v", model.statusMessage, model.statusUndoCb != nil, model.statusExpiresAt)
		}
	})

	t.Run("superseded transaction degrades", func(t *testing.T) {
		model, transactionID := newRecoveryFeedbackEnv(t)
		fresh := "Newer title"
		projectID := model.currentProject.ID
		taskID := model.currentProject.Tasks[0].ID
		if _, err := model.appService.Edit(app.EditRequest{
			Locator: app.TaskLocator{ProjectID: projectID, TaskID: taskID},
			Title:   &fresh,
		}); err != nil {
			t.Fatalf("superseding Edit: %v", err)
		}
		model.setRecoveryFeedback("Older update", transactionID, false)
		if model.statusMessage != "Older update" || model.statusUndoCb != nil {
			t.Fatalf("superseded transaction advertised undo: status=%q callback=%v", model.statusMessage, model.statusUndoCb != nil)
		}
	})

	t.Run("missing ledger degrades", func(t *testing.T) {
		model := newTestAppModel()
		model.runtime = newTestConfigAt(t.TempDir())
		model.setRecoveryFeedback("Saved task", "unrecorded-transaction", false)
		if model.statusMessage != "Saved task" || model.statusUndoCb != nil {
			t.Fatalf("missing ledger advertised undo: status=%q callback=%v", model.statusMessage, model.statusUndoCb != nil)
		}
	})

	t.Run("unreadable ledger degrades", func(t *testing.T) {
		dataDir := t.TempDir()
		ledgerDir := config.UndoLedgerDir(dataDir)
		if err := os.MkdirAll(ledgerDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ledgerDir, "ledger.json"), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		model := newTestAppModel()
		model.runtime = newTestConfigAt(dataDir)
		model.setRecoveryFeedback("Saved task", "corrupt-ledger-transaction", false)
		if model.statusMessage != "Saved task" || model.statusUndoCb != nil {
			t.Fatalf("unreadable ledger advertised undo: status=%q callback=%v", model.statusMessage, model.statusUndoCb != nil)
		}
	})
}

func TestRecoveryFeedbackAdvertisesOriginOnlyWhenRecorded(t *testing.T) {
	model, transactionID := newRecoveryFeedbackEnv(t)
	model.setRecoveryFeedback("Updated task", transactionID, false)
	if strings.Contains(model.statusMessage, "origin") {
		t.Fatalf("unrecorded relocation advertised origin: %q", model.statusMessage)
	}
	model.setRecoveryFeedback("Updated task", transactionID, true)
	if model.statusMessage != "Edit Task · u undo · Ctrl+O origin" {
		t.Fatalf("recorded relocation status = %q", model.statusMessage)
	}
}

func TestRemainingMutationFamiliesRequireMatchingLedgerTransaction(t *testing.T) {
	type familyCase struct {
		name     string
		fallback string
		deliver  func(*AppModel, string)
	}
	families := []familyCase{
		{name: "project", fallback: "Updated project", deliver: func(model *AppModel, transactionID string) {
			model.handleProjectMessages(ProjectUpdatedMsg{TransactionID: transactionID})
		}},
		{name: "note", fallback: "Updated note", deliver: func(model *AppModel, transactionID string) {
			model.reconcileNoteResult(ProjectNoteMutationResultMsg{Result: app.NoteBatchResult{TransactionID: transactionID}, SuccessMessage: "Updated note"})
		}},
		{name: "checklist", fallback: "Checklist item updated", deliver: func(model *AppModel, transactionID string) {
			model.handleChecklistItemEdited(ChecklistItemEditedMsg{TransactionID: transactionID})
		}},
		{name: "comment", fallback: "Comment updated", deliver: func(model *AppModel, transactionID string) {
			model.handleCommentEdited(CommentEditedMsg{TransactionID: transactionID})
		}},
		{name: "attachment", fallback: "Attach File", deliver: func(model *AppModel, transactionID string) {
			model.handleTransactionResult(TransactionResultMsg{Action: "Attach File", Result: app.TaskResult{TransactionID: transactionID}})
		}},
		{name: "tag", fallback: "Tagged 1 task", deliver: func(model *AppModel, transactionID string) {
			model.handleBatchTransactionResult(BatchTransactionResultMsg{Action: "Tag Tasks", Summary: "Tagged 1 task", TransactionID: transactionID})
		}},
	}

	for _, family := range families {
		family := family
		t.Run(family.name+" matching", func(t *testing.T) {
			model, transactionID := newRecoveryFeedbackEnv(t)
			family.deliver(model, transactionID)
			if model.statusUndoCb == nil || model.statusTransactionID != transactionID {
				t.Fatalf("matching %s result did not install correlated recovery: status=%q transaction=%q callback=%v", family.name, model.statusMessage, model.statusTransactionID, model.statusUndoCb != nil)
			}
		})
		t.Run(family.name+" mismatching", func(t *testing.T) {
			model, _ := newRecoveryFeedbackEnv(t)
			family.deliver(model, "mismatching-transaction")
			if model.statusMessage != family.fallback || model.statusUndoCb != nil || model.statusTransactionID != "" {
				t.Fatalf("mismatching %s result did not degrade: status=%q transaction=%q callback=%v", family.name, model.statusMessage, model.statusTransactionID, model.statusUndoCb != nil)
			}
		})
	}
}

func TestSubviewAdapterPreservesTransactionCorrelation(t *testing.T) {
	transactionID := "subview-transaction"
	commit := func() tea.Msg {
		return TransactionResultMsg{Result: app.TaskResult{TransactionID: transactionID}}
	}
	tests := []struct {
		name    string
		success func() tea.Msg
		id      func(tea.Msg) string
	}{
		{name: "comment", success: func() tea.Msg { return CommentAddedMsg{} }, id: func(msg tea.Msg) string { return msg.(CommentAddedMsg).TransactionID }},
		{name: "checklist", success: func() tea.Msg { return ChecklistItemAddedMsg{} }, id: func(msg tea.Msg) string { return msg.(ChecklistItemAddedMsg).TransactionID }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.id(adaptSubviewCommit(commit, test.success)()); got != transactionID {
				t.Fatalf("transaction ID = %q, want %q", got, transactionID)
			}
		})
	}
}
