package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
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
		if cmd == nil || model.feedback.undoCb == nil {
			t.Fatal("matching newest transaction did not install recovery callback")
		}
		if model.feedback.message != "Edit Task · u undo" {
			t.Fatalf("status = %q, want canonical recovery feedback", model.feedback.message)
		}
		if model.feedback.transactionID != transactionID || !time.Now().Before(model.feedback.expiresAt) {
			t.Fatal("matching recovery feedback did not retain its ID and six-second window")
		}
	})

	t.Run("missing transaction degrades", func(t *testing.T) {
		model, _ := newRecoveryFeedbackEnv(t)
		if cmd := model.setRecoveryFeedback("Updated task", "", false); cmd == nil {
			t.Fatal("plain success did not receive its normal timeout")
		}
		if model.feedback.message != "Updated task" || model.feedback.undoCb != nil || !model.feedback.expiresAt.IsZero() {
			t.Fatalf("missing-ID feedback did not degrade safely: status=%q callback=%v expiry=%v", model.feedback.message, model.feedback.undoCb != nil, model.feedback.expiresAt)
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
		if model.feedback.message != "Older update" || model.feedback.undoCb != nil {
			t.Fatalf("superseded transaction advertised undo: status=%q callback=%v", model.feedback.message, model.feedback.undoCb != nil)
		}
	})

	t.Run("missing ledger degrades", func(t *testing.T) {
		model := newTestAppModel()
		model.runtime = newTestConfigAt(t.TempDir())
		model.setRecoveryFeedback("Saved task", "unrecorded-transaction", false)
		if model.feedback.message != "Saved task" || model.feedback.undoCb != nil {
			t.Fatalf("missing ledger advertised undo: status=%q callback=%v", model.feedback.message, model.feedback.undoCb != nil)
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
		if model.feedback.message != "Saved task" || model.feedback.undoCb != nil {
			t.Fatalf("unreadable ledger advertised undo: status=%q callback=%v", model.feedback.message, model.feedback.undoCb != nil)
		}
	})
}

func TestRecoveryFeedbackAdvertisesOriginOnlyWhenRecorded(t *testing.T) {
	model, transactionID := newRecoveryFeedbackEnv(t)
	model.setRecoveryFeedback("Updated task", transactionID, false)
	if strings.Contains(model.feedback.message, "origin") {
		t.Fatalf("unrecorded relocation advertised origin: %q", model.feedback.message)
	}
	model.setRecoveryFeedback("Updated task", transactionID, true)
	if model.feedback.message != "Edit Task · u undo · Ctrl+O origin" {
		t.Fatalf("recorded relocation status = %q", model.feedback.message)
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
			model.handleProjectMessages(projecthub.ProjectUpdatedMsg{TransactionID: transactionID})
		}},
		{name: "note", fallback: "Updated note", deliver: func(model *AppModel, transactionID string) {
			model.reconcileNoteResult(notes.ProjectNoteMutationResultMsg{Result: app.NoteBatchResult{TransactionID: transactionID}, SuccessMessage: "Updated note"})
		}},
		{name: "comment", fallback: "Comment updated", deliver: func(model *AppModel, transactionID string) {
			model.handleCommentEdited(taskdetail.CommentEditedMsg{TransactionID: transactionID})
		}},
		{name: "attachment", fallback: "Attach File", deliver: func(model *AppModel, transactionID string) {
			model.handleTransactionResult(coordinator.TransactionResultMsg{Action: "Attach File", Result: app.TaskResult{TransactionID: transactionID}})
		}},
		{name: "tag", fallback: "Tagged 1 task", deliver: func(model *AppModel, transactionID string) {
			model.handleBatchTransactionResult(coordinator.BatchTransactionResultMsg{Action: "Tag Tasks", Summary: "Tagged 1 task", TransactionID: transactionID})
		}},
	}

	for _, family := range families {
		family := family
		t.Run(family.name+" matching", func(t *testing.T) {
			model, transactionID := newRecoveryFeedbackEnv(t)
			family.deliver(model, transactionID)
			if model.feedback.undoCb == nil || model.feedback.transactionID != transactionID {
				t.Fatalf("matching %s result did not install correlated recovery: status=%q transaction=%q callback=%v", family.name, model.feedback.message, model.feedback.transactionID, model.feedback.undoCb != nil)
			}
		})
		t.Run(family.name+" mismatching", func(t *testing.T) {
			model, _ := newRecoveryFeedbackEnv(t)
			family.deliver(model, "mismatching-transaction")
			if model.feedback.message != family.fallback || model.feedback.undoCb != nil || model.feedback.transactionID != "" {
				t.Fatalf("mismatching %s result did not degrade: status=%q transaction=%q callback=%v", family.name, model.feedback.message, model.feedback.transactionID, model.feedback.undoCb != nil)
			}
		})
	}
}
