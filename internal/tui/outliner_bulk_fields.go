package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// crossApplyBulkDueDate sets or clears the due date on every selected task in the
// cross-project outliner through the application-layer BulkCoordinator. The batch
// commits ONE asynchronous cross-project transaction with a workspace-ledger undo
// entry; the shell owns reload, selection-clear, and recent-visit repair via
// finalizeBatchResult. Per-view summary wording and date-only semantics are
// preserved.
func (m *OutlinerModel) crossApplyBulkDueDate(refs []TaskProjectRef, days int, isClear bool) (tea.Model, tea.Cmd) {
	locators := taskLocatorsFromTaskRefs(validTaskProjectRefs(refs))
	if len(locators) == 0 {
		return m, nil
	}
	if isClear {
		cmd := m.bulk.ClearDueDateLocators(locators)
		summary := fmt.Sprintf("Cleared due date on %d tasks", len(locators))
		return m, finalizeLocatedBatchResult(cmd, locators, summary)
	}
	dueDate, desc := dueDateFromDays(days, false)
	cmd := m.bulk.SetDueDateLocators(locators, *dueDate, false)
	summary := fmt.Sprintf("Due date set to %s on %d tasks", desc, len(locators))
	return m, finalizeLocatedBatchResult(cmd, locators, summary)
}

// crossApplyBulkSnooze snoozes (or clears the snooze on) every selected task in
// the cross-project outliner through the BulkCoordinator, committing ONE
// asynchronous cross-project transaction with a workspace-ledger undo entry.
func (m *OutlinerModel) crossApplyBulkSnooze(refs []TaskProjectRef, snoozeDate *time.Time, desc string) (tea.Model, tea.Cmd) {
	locators := taskLocatorsFromTaskRefs(validTaskProjectRefs(refs))
	if len(locators) == 0 {
		return m, nil
	}
	cmd := m.bulk.SnoozeLocators(locators, snoozeDate)
	summary := fmt.Sprintf("Snoozed %d tasks %s", len(locators), desc)
	if snoozeDate == nil {
		summary = fmt.Sprintf("Cleared snooze on %d tasks", len(locators))
	}
	return m, finalizeLocatedBatchResult(cmd, locators, summary)
}
