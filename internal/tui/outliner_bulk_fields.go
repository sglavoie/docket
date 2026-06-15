package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *OutlinerModel) crossApplyBulkDueDate(refs []TaskProjectRef, days int, isClear bool) (tea.Model, tea.Cmd) {
	dueDate, desc := dueDateFromDays(days, isClear)
	count, groups, cmd := m.applyCrossBulkTaskField(refs, "Set Due Date", func(t *models.Task) {
		if dueDate != nil {
			d := *dueDate
			t.DueDate = &d
		} else {
			t.DueDate = nil
		}
		t.HasTime = false
	})
	if cmd != nil {
		return m, cmd
	}

	msg := fmt.Sprintf("Due date set to %s on %d tasks", desc, count)
	if isClear {
		msg = fmt.Sprintf("Cleared due date on %d tasks", count)
	}
	m.finishCrossBulkTaskField(refs, groups, "Set Due Date", msg)
	return m, nil
}

func (m *OutlinerModel) crossApplyBulkSnooze(refs []TaskProjectRef, snoozeDate *time.Time, desc string) (tea.Model, tea.Cmd) {
	count, groups, cmd := m.applyCrossBulkTaskField(refs, "Snooze", func(t *models.Task) {
		if snoozeDate != nil {
			d := *snoozeDate
			t.SnoozedUntil = &d
		} else {
			t.SnoozedUntil = nil
		}
	})
	if cmd != nil {
		return m, cmd
	}

	msg := fmt.Sprintf("Snoozed %d tasks %s", count, desc)
	if snoozeDate == nil {
		msg = fmt.Sprintf("Cleared snooze on %d tasks", count)
	}
	m.finishCrossBulkTaskField(refs, groups, "Snooze", msg)
	return m, nil
}

func (m *OutlinerModel) applyCrossBulkTaskField(
	refs []TaskProjectRef,
	undoAction string,
	mutate func(*models.Task),
) (int, []TaskProjectGroup, tea.Cmd) {
	refs = validTaskProjectRefs(refs)
	if len(refs) == 0 {
		return 0, nil, nil
	}

	count, err := taskOpsApplyBulk(m, refs, undoAction, func(project *models.Project, taskID string) bool {
		return project.MutateTask(taskID, mutate)
	})
	if err != nil {
		return count, nil, func() tea.Msg { return ErrorMsg{Err: err} }
	}
	return count, groupTaskProjectRefsByProject(refs), nil
}

func (m *OutlinerModel) finishCrossBulkTaskField(refs []TaskProjectRef, groups []TaskProjectGroup, undoAction, msg string) {
	m.crossUndoCtx.Set(undoAction, msg)
	m.crossUndoCtx.ProjectIDs = taskProjectGroupIDs(groups)
	m.crossUndoCtx.TaskIDs = taskIDsFromTaskRefs(refs)
	m.clearMultiSelect()
	m.setStatus(msg)
}

func taskProjectGroupIDs(groups []TaskProjectGroup) []string {
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.Project == nil {
			continue
		}
		ids = append(ids, group.Project.ID)
	}
	return ids
}
