package coordinator

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// BulkCoordinator backs the bulk-edit / bulk-capture selection overlays.
type BulkCoordinator struct {
	bulk BulkMutator
}

// TransitionLocators moves a project-qualified selection as one transaction.
func (c BulkCoordinator) TransitionLocators(locators []app.TaskLocator, target models.WorkflowState, summary func(changed int) string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.bulk.BulkTransition(locators, target)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		msg := ""
		if summary != nil {
			msg = summary(len(res.Tasks))
		}
		return BatchTransactionResultMsg{
			Action:            "Bulk Transition",
			Tasks:             res.Tasks,
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           msg,
			TransactionID:     res.TransactionID,
		}
	}
}

// DeleteLocators permanently removes every selected subtree as one transaction.
// The caller must have already confirmed the destructive intent; Confirm is set
// here so the application layer accepts the batch. Any artifacts (e.g. a config
// store with the removed tasks' durable planning refs cleaned) ride the same
// transaction. The result carries every removed task UUID for the shared
// transaction result.
func (c BulkCoordinator) DeleteLocators(locators []app.TaskLocator, artifacts ...storage.ArtifactChange) tea.Cmd {
	req := app.BulkDeleteRequest{TaskLocators: locators, Confirm: true}
	return func() tea.Msg {
		res, err := c.bulk.BulkDelete(req, artifacts...)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		removedCount := len(res.RemovedTaskIDs)
		if len(res.RemovedTaskLocators) > 0 {
			removedCount = len(res.RemovedTaskLocators)
		}
		return BatchTransactionResultMsg{
			Action:            "Bulk Delete",
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           fmt.Sprintf("Deleted %d task(s)", removedCount),
			TransactionID:     res.TransactionID,
			DueEffect:         CommittedDueEffect{Removed: res.RemovedTaskLocators},
		}
	}
}

func (c BulkCoordinator) SetPriorityLocators(locators []app.TaskLocator, priority int) tea.Cmd {
	return runBatch("Bulk Set Priority", func() ([]string, string, string, error) {
		res, err := c.bulk.BulkSetPriority(locators, priority)
		if err != nil {
			return nil, "", "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Set priority on %d task(s)", len(res.Tasks)), res.TransactionID, nil
	})
}

func (c BulkCoordinator) SetDueDateLocators(locators []app.TaskLocator, due time.Time) tea.Cmd {
	return runBatchWithDueEffect("Bulk Set Due Date", func() ([]string, string, string, CommittedDueEffect, error) {
		var (
			res app.BulkResult
			err error
		)
		if DueDateIsToday(&due) {
			res, err = c.bulk.BulkSetDueDateAndPlanToday(locators, due)
		} else {
			res, err = c.bulk.BulkSetDueDate(locators, due)
		}
		if err != nil {
			return nil, "", "", CommittedDueEffect{}, err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Set due date on %d task(s)", len(res.Tasks)), res.TransactionID,
			CommittedDueEffect{Edits: res.DueEdits}, nil
	})
}

func (c BulkCoordinator) ClearDueDateLocators(locators []app.TaskLocator) tea.Cmd {
	return runBatchWithDueEffect("Bulk Clear Due Date", func() ([]string, string, string, CommittedDueEffect, error) {
		res, err := c.bulk.BulkClearDueDate(locators)
		if err != nil {
			return nil, "", "", CommittedDueEffect{}, err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Cleared due date on %d task(s)", len(res.Tasks)), res.TransactionID,
			CommittedDueEffect{Edits: res.DueEdits}, nil
	})
}

func (c BulkCoordinator) DeferLocators(locators []app.TaskLocator, until *time.Time) tea.Cmd {
	return runBatch("Bulk Defer", func() ([]string, string, string, error) {
		res, err := c.bulk.BulkDefer(locators, until)
		if err != nil {
			return nil, "", "", err
		}
		verb := "Deferred"
		if until == nil {
			verb = "Cleared defer on"
		}
		return res.TouchedProjectIDs, fmt.Sprintf("%s %d task(s)", verb, len(res.Tasks)), res.TransactionID, nil
	})
}

// MoveToProjectLocators relocates every selected subtree into one destination
// project as one transaction with one undo entry. summary receives the number
// of subtrees the batch actually moved, which is the selection collapsed to its
// maximal subtrees: a selected descendant of a selected ancestor traveled
// inside that ancestor and is not counted twice. The count is therefore
// independent of how many source projects the selection spanned, so one status
// sentence reads correctly for N tasks across M sources.
func (c BulkCoordinator) MoveToProjectLocators(
	locators []app.TaskLocator,
	destProjectID string,
	summary func(moved int) string,
) tea.Cmd {
	return runBatchWithDueEffect("Move to Project", func() ([]string, string, string, CommittedDueEffect, error) {
		res, err := c.bulk.BulkMoveToProject(locators, destProjectID)
		if err != nil {
			return nil, "", "", CommittedDueEffect{}, err
		}
		text := ""
		if summary != nil {
			text = summary(len(res.Tasks))
		}
		return res.TouchedProjectIDs, text, res.TransactionID, CommittedDueEffect{Moves: res.Moves}, nil
	})
}

// ApplyPlanningLocators applies one planning intent to every selected task as a
// single transaction.
func (c BulkCoordinator) ApplyPlanningLocators(
	locators []app.TaskLocator,
	action app.PlanningAction,
	summary string,
) tea.Cmd {
	return runBatch("Bulk Planning", func() ([]string, string, string, error) {
		res, err := c.bulk.ApplyPlanningBulk(app.PlanningBulkRequest{
			Targets: locators,
			Action:  action,
		})
		if err != nil {
			return nil, "", "", err
		}
		if summary == "" {
			summary = fmt.Sprintf("Updated %d task(s)", len(res.Tasks))
		}
		return res.TouchedProjectIDs, summary, res.TransactionID, nil
	})
}
