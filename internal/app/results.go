package app

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// TaskAddressChange is one committed source-to-destination task address. A move
// keeps a task's UUID and changes the project that qualifies it, so a caller
// holding project-qualified session state — a remembered clock, a frozen
// selection, a jump frame — re-addresses that state from this explicit map
// rather than from a UUID, which is unique only inside its own project.
//
// A move reports one entry per relocated task, descendants included, because the
// descendants change address without ever being named by the request.
type TaskAddressChange struct {
	From TaskLocator
	To   TaskLocator
}

// taskAddressChanges projects the reference-maintenance map a cross-project move
// already builds into the committed address map results carry. The order is
// deterministic (source project, then source task) so two runs of the same move
// report the same list rather than Go's map order.
func taskAddressChanges(moved map[models.TaskReference]models.TaskReference) []TaskAddressChange {
	if len(moved) == 0 {
		return nil
	}
	changes := make([]TaskAddressChange, 0, len(moved))
	for from, to := range moved {
		changes = append(changes, TaskAddressChange{
			From: TaskLocator{ProjectID: from.ProjectID, TaskID: from.TaskID},
			To:   TaskLocator{ProjectID: to.ProjectID, TaskID: to.TaskID},
		})
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].From.ProjectID != changes[j].From.ProjectID {
			return changes[i].From.ProjectID < changes[j].From.ProjectID
		}
		return changes[i].From.TaskID < changes[j].From.TaskID
	})
	return changes
}

// taskResultLocators names every task a result reports, in result order. It is
// what turns a committed bulk outcome into the project-qualified addresses a
// caller's own session state is keyed by.
func taskResultLocators(results []TaskResult) []TaskLocator {
	if len(results) == 0 {
		return nil
	}
	locators := make([]TaskLocator, 0, len(results))
	for _, result := range results {
		locators = append(locators, TaskLocator{ProjectID: result.ProjectID, TaskID: result.TaskID})
	}
	return locators
}

// TaskResult is the presentation-neutral outcome of a task mutation. It carries
// only stable, adapter-agnostic identifiers and derived state so a CLI or TUI
// caller can render its own output without reaching back into the domain model.
type TaskResult struct {
	// TaskID is the task's stable UUID.
	TaskID string
	// ShortID is the task's per-project short numeric ID.
	ShortID int
	// ProjectID is the owning project's stable ID.
	ProjectID string
	// Title is the task's current title.
	Title string
	// Revision is the project's new canonical revision after the commit.
	Revision string
	// TransactionID identifies the atomic durable transaction that produced this
	// result. Adapters use it to correlate recovery affordances with the newest
	// workspace-ledger entry.
	TransactionID string
	// StateEffect describes the committed lifecycle entry, including creation.
	StateEffect *TaskStateEffect
}

// newTaskResult builds a TaskResult from a resolved task/project pair and the
// revision the commit produced.
func newTaskResult(ref TaskRef, revision, transactionID string) TaskResult {
	return TaskResult{
		TaskID:        ref.Task.ID,
		ShortID:       ref.Task.ShortID,
		ProjectID:     ref.Project.ID,
		Title:         ref.Task.Title,
		Revision:      revision,
		TransactionID: transactionID,
		StateEffect:   taskStateEffect(ref),
	}
}
