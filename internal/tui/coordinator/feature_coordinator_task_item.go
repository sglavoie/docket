package coordinator

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// TaskItemCoordinator backs the single-task outliner mutation handlers, parallel
// to BulkCoordinator but for one task at a time. The attribute seams
// attribute operations resolve to a TransactionResultMsg (a
// single-task commit, reloaded by handleTransactionResult); the reorder seams
// (MoveToTop/MoveToBottom) resolve to a BatchTransactionResultMsg because they
// reorder a whole project's task slice, reloaded by handleBatchTransactionResult.
// Both messages flow through the same save/reload path. The shell starts a
// cross-project reload only while the Outliner is routable and otherwise defers
// it until the view reappears. Every task argument is project-qualified so a
// coordinator cannot accidentally fall back to workspace iteration order.
type TaskItemCoordinator struct {
	task TaskItemMutator
}

// Wired reports whether an application mutator was injected. Isolated Outliner
// models built for focused rendering and navigation tests carry the zero
// coordinator, and the few structural handlers that keep an in-memory fallback
// for them ask this before dispatching a transaction.
func (c TaskItemCoordinator) Wired() bool { return c.task != nil }

// ExecuteOperation runs title/description edits, clipboard projections, child
// creation, and direct state/metadata actions through one application request.
// Mutations use the batch result path even for one target, so single and bulk
// operations share identical reload and transaction semantics.
func (c TaskItemCoordinator) ExecuteOperation(req app.TaskOperationRequest, action, summary string) tea.Cmd {
	return func() tea.Msg {
		result, err := c.task.ExecuteTaskOperation(req)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		if result.Clipboard != nil {
			return TaskClipboardProjectedMsg{Projection: *result.Clipboard}
		}
		if summary == "" {
			summary = action
			if len(result.Tasks) > 1 {
				summary = fmt.Sprintf("%s on %d task(s)", action, len(result.Tasks))
			}
		}
		return BatchTransactionResultMsg{
			Action:            action,
			TouchedProjectIDs: result.TouchedProjectIDs,
			Summary:           summary,
			TransactionID:     result.TransactionID,
		}
	}
}

// MoveToTop moves one task to the top of its siblings as one project-scoped
// transaction, emitting a BatchTransactionResultMsg so the touched project is
// reloaded through the shared batch path.
func (c TaskItemCoordinator) MoveToTop(task taskref.TaskIdentity) tea.Cmd {
	return runBatch("Move to Top", func() ([]string, string, string, error) {
		res, err := c.task.MoveTaskToTop(task.Locator())
		if err != nil {
			return nil, "", "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d to top", res.ShortID), res.TransactionID, nil
	})
}

// MoveToBottom moves one task to the bottom of its siblings as one project-scoped
// transaction (see MoveToTop).
func (c TaskItemCoordinator) MoveToBottom(task taskref.TaskIdentity) tea.Cmd {
	return runBatch("Move to Bottom", func() ([]string, string, string, error) {
		res, err := c.task.MoveTaskToBottom(task.Locator())
		if err != nil {
			return nil, "", "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d to bottom", res.ShortID), res.TransactionID, nil
	})
}

// MoveUp swaps one task with its previous model-order sibling as one
// project-scoped transaction (see MoveToTop for the reload path).
func (c TaskItemCoordinator) MoveUp(task taskref.TaskIdentity) tea.Cmd {
	return runBatch("Move Up", func() ([]string, string, string, error) {
		res, err := c.task.MoveTaskUp(task.Locator())
		if err != nil {
			return nil, "", "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d up", res.ShortID), res.TransactionID, nil
	})
}

// MoveDown swaps one task with its next model-order sibling as one project-scoped
// transaction.
func (c TaskItemCoordinator) MoveDown(task taskref.TaskIdentity) tea.Cmd {
	return runBatch("Move Down", func() ([]string, string, string, error) {
		res, err := c.task.MoveTaskDown(task.Locator())
		if err != nil {
			return nil, "", "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d down", res.ShortID), res.TransactionID, nil
	})
}

// ReorderSiblings swaps the two explicit sibling identities as one
// project-scoped transaction. Passing the adjacent visible target lets filtered
// and collapsed Outliner projections preserve hidden siblings' relative order.
func (c TaskItemCoordinator) ReorderSiblings(task, target taskref.TaskIdentity) tea.Cmd {
	return runBatch("Reorder Siblings", func() ([]string, string, string, error) {
		res, err := c.task.ReorderSiblingTasks(task.Locator(), target.Locator())
		if err != nil {
			return nil, "", "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Reordered #%d", res.ShortID), res.TransactionID, nil
	})
}

// ReorderSiblingOrder permutes the named siblings into the given top-to-bottom
// order as one project-scoped transaction. It is the whole-chain form of
// ReorderSiblings: the caller lists exactly the visible peers, so hidden
// siblings between them keep their durable slots.
func (c TaskItemCoordinator) ReorderSiblingOrder(moved taskref.TaskIdentity, order []taskref.TaskIdentity, edge string) tea.Cmd {
	return runBatch("Reorder Siblings", func() ([]string, string, string, error) {
		res, err := c.task.ReorderSiblingOrder(moved.Locator(), taskIdentityLocators(order))
		if err != nil {
			return nil, "", "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d to %s", res.ShortID, edge), res.TransactionID, nil
	})
}

// ReorderPinnedOrder rewrites the named pinned chain into the given
// top-to-bottom display order as one transaction, which may span projects.
func (c TaskItemCoordinator) ReorderPinnedOrder(moved taskref.TaskIdentity, order []taskref.TaskIdentity, edge string) tea.Cmd {
	return runBatch("Reorder Pinned", func() ([]string, string, string, error) {
		res, err := c.task.ReorderPinnedTasksOrder(moved.Locator(), taskIdentityLocators(order))
		if err != nil {
			return nil, "", "", err
		}
		return res.ProjectIDs, fmt.Sprintf("Moved pinned #%d to %s", res.ShortID, edge), res.TransactionID, nil
	})
}

func taskIdentityLocators(order []taskref.TaskIdentity) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(order))
	for _, identity := range order {
		locators = append(locators, identity.Locator())
	}
	return locators
}

// ReorderPinned swaps the pin order of two pinned tasks as one transaction.
// task is the identity that must end up displayed first, because the pinned
// order is PinnedAt-descending and equal timestamps resolve in its favor.
func (c TaskItemCoordinator) ReorderPinned(task, target taskref.TaskIdentity) tea.Cmd {
	return runBatch("Reorder Pinned", func() ([]string, string, string, error) {
		res, err := c.task.ReorderPinnedTasks(task.Locator(), target.Locator())
		if err != nil {
			return nil, "", "", err
		}
		return res.ProjectIDs, fmt.Sprintf("Reordered pinned #%d", res.ShortID), res.TransactionID, nil
	})
}

// Indent nests one task under the task model-above it as one project-scoped
// transaction.
func (c TaskItemCoordinator) Indent(task taskref.TaskIdentity) tea.Cmd {
	return runBatch("Indent", func() ([]string, string, string, error) {
		res, err := c.task.IndentTask(task.Locator())
		if err != nil {
			return nil, "", "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Indented #%d", res.ShortID), res.TransactionID, nil
	})
}

// Outdent moves one task to become a sibling of its parent as one project-scoped
// transaction.
func (c TaskItemCoordinator) Outdent(task taskref.TaskIdentity) tea.Cmd {
	return runBatch("Outdent", func() ([]string, string, string, error) {
		res, err := c.task.OutdentTask(task.Locator())
		if err != nil {
			return nil, "", "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Outdented #%d", res.ShortID), res.TransactionID, nil
	})
}

// Transition moves one task to a target workflow state as one transaction.
func (c TaskItemCoordinator) Transition(task taskref.TaskIdentity, target models.WorkflowState) tea.Cmd {
	return func() tea.Msg {
		res, err := c.task.Transition(app.StateRequest{
			Locator: task.Locator(),
			Target:  target,
		})
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return TransactionResultMsg{
			Action: "Change State", Result: res,
			PlanningChanged: true,
		}
	}
}

// Edit applies content/form changes to one task as one transaction.
func (c TaskItemCoordinator) Edit(task taskref.TaskIdentity, req app.EditRequest) tea.Cmd {
	req.Locator = task.Locator()
	return runTask("Edit Task", func() (app.TaskResult, error) {
		return c.task.Edit(req)
	})
}

// EditTaskLine commits one parsed row edit without mutating any view-owned task
// first. A rejected command returns the source identity so the shell can restore
// input immediately on the still-active editor.
func (c TaskItemCoordinator) EditTaskLine(req app.TaskLineEditRequest, relocation TransactionRelocation) tea.Cmd {
	return func() tea.Msg {
		result, err := c.task.EditTaskLine(req)
		source := taskref.TaskIdentity{ProjectID: req.Source.ProjectID, TaskID: req.Source.TaskID}
		if err != nil {
			return TaskLineEditFailedMsg{Source: source, Err: err}
		}
		return TaskLineEditedMsg{Result: result, Source: source, Relocation: relocation}
	}
}

// Capture creates a new task in a project as one transaction.
func (c TaskItemCoordinator) Capture(req app.CaptureRequest) tea.Cmd {
	return func() tea.Msg {
		res, err := c.task.Capture(req)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return TransactionResultMsg{
			Action: "Add Task", Result: res,
			PlanningChanged: req.Destination.IsPlanning(),
		}
	}
}

// SetPriority sets one task's priority as one transaction.
func (c TaskItemCoordinator) SetPriority(task taskref.TaskIdentity, priority int) tea.Cmd {
	return runTask("Set Priority", func() (app.TaskResult, error) {
		return c.task.SetPriority(task.Locator(), priority)
	})
}

// SetDueDate sets or clears (nil due) one task's due date as one transaction.
func (c TaskItemCoordinator) SetDueDate(task taskref.TaskIdentity, due *time.Time) tea.Cmd {
	return func() tea.Msg {
		var (
			res app.TaskResult
			err error
		)
		planningChanged := DueDateIsToday(due)
		if planningChanged {
			res, err = c.task.SetDueDateAndPlanToday(task.Locator(), *due)
		} else {
			res, err = c.task.SetDueDate(task.Locator(), due)
		}
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return TransactionResultMsg{
			Action: "Set Due Date", Result: res, PlanningChanged: planningChanged,
		}
	}
}

// Defer hides one task until until, or clears the deferral when until is nil, as
// one transaction.
func (c TaskItemCoordinator) Defer(task taskref.TaskIdentity, until *time.Time) tea.Cmd {
	return runTask("Defer", func() (app.TaskResult, error) {
		return c.task.Defer(task.Locator(), until)
	})
}

// SetPinned pins or unpins one task as one transaction.
func (c TaskItemCoordinator) SetPinned(task taskref.TaskIdentity, pinned bool) tea.Cmd {
	return runTask("Toggle Pin", func() (app.TaskResult, error) {
		return c.task.SetPinned(task.Locator(), pinned)
	})
}

// MarkReviewed acknowledges one task awaiting review as one transaction. The
// application layer rechecks eligibility against its own clock, so a row the
// surface admitted from a stale draw is refused there rather than committed.
func (c TaskItemCoordinator) MarkReviewed(task taskref.TaskIdentity) tea.Cmd {
	return runTask("Mark Reviewed", func() (app.TaskResult, error) {
		return c.task.MarkReviewed(task.Locator())
	})
}

// SetResumeNote writes, replaces, or clears one task's resume note as one
// transaction. The empty note is the clear, so this is one seam rather than a
// set/clear pair.
//
// A note equal to the stored one issues no transaction at all and says so: an
// empty commit would still write a ledger entry `u` could take back, and a
// prompt closed on unchanged text is not something to undo.
func (c TaskItemCoordinator) SetResumeNote(task taskref.TaskIdentity, note string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.task.SetResumeNote(task.Locator(), note)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		if !res.Changed {
			return status.InfoStatusMsg("Resume note unchanged")
		}
		action := "Set Resume Note"
		if note == "" {
			action = "Clear Resume Note"
		}
		return TransactionResultMsg{Action: action, Result: res.Task}
	}
}

// Reparent changes one task's parent within its owning project as one transaction.
func (c TaskItemCoordinator) Reparent(task taskref.TaskIdentity, newParentID *string) tea.Cmd {
	return runTask("Reparent Task", func() (app.TaskResult, error) {
		return c.task.Reparent(task.Locator(), newParentID)
	})
}

// DeleteTask permanently removes one task's subtree as one transaction, folding
// the caller-supplied durable planning-ref artifacts into it. The result carries
// every removed task UUID (mirroring BulkCoordinator.DeleteLocators).
func (c TaskItemCoordinator) DeleteTask(task taskref.TaskIdentity, artifacts []storage.ArtifactChange, summary func(app.DeleteResult) string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.task.Delete(app.DeleteRequest{Locator: task.Locator(), Confirm: true}, artifacts...)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		msg := ""
		if summary != nil {
			msg = summary(res)
		}
		return BatchTransactionResultMsg{
			Action:            "Delete Task",
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           msg,
			TransactionID:     res.TransactionID,
		}
	}
}

// CommitProject commits an already-mutated project through the application
// layer, resolving to a
// BatchTransactionResultMsg (or TransactionErrorMsg on rejection). It is the
// project-scoped analog of the *Workflow single-task methods, backing the
// ProjectTree and Outliner structural / multi-field mutations (reorder,
// indent/outdent, reparent, bulk edits, section moves, paste) whose in-memory
// change spans the whole project rather than one field of one task. The view
// mutated the project synchronously (so its cursor/tree logic and unit tests see
// the change); this seam commits that state durably and the shell reloads every
// touched project. summary sets the status-line text; an empty summary leaves the
// view's synchronously-set status intact (the shell skips an empty success line).
func (c TaskItemCoordinator) CommitProject(project *models.Project, opts app.ProjectCommitOptions, summary string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.task.CommitProject(project, opts)
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return BatchTransactionResultMsg{
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           summary,
			TransactionID:     res.TransactionID,
		}
	}
}
