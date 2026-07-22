package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// repeatableAction stores a repeatable prefix-chord action for the . key.
type repeatableAction struct {
	label string
	apply func(v prefixableView) tea.Cmd
}

// RecordRepeatableActionMsg is sent after completing a single-task mutation
// via a prefix chord. AppModel stores the action so . can replay it.
type RecordRepeatableActionMsg struct {
	Action *repeatableAction
}

func recordRepeatPriority(priority int) tea.Cmd {
	p := priority
	return func() tea.Msg {
		return RecordRepeatableActionMsg{
			Action: &repeatableAction{
				label: fmt.Sprintf("Set Priority %d", p),
				apply: func(v prefixableView) tea.Cmd {
					task := v.getCursorTask()
					project := v.getProject()
					if task == nil || project == nil {
						return nil
					}
					if task.Priority == p {
						return nil
					}
					cmd := v.singleTaskCoordinator().SetPriority(taskIdentityFor(task, project), p)
					msg := cmd()
					if em, ok := msg.(TransactionErrorMsg); ok {
						return func() tea.Msg { return ErrorMsg{Err: em.Err} }
					}
					v.setMutationFlash(priorityMutationFlashBg(p))
					return tea.Batch(func() tea.Msg { return msg }, mutationFlashTimeout())
				},
			},
		}
	}
}

func recordRepeatQuickDueDate(days int) tea.Cmd {
	d := days
	return func() tea.Msg {
		return RecordRepeatableActionMsg{
			Action: &repeatableAction{
				label: "Set Due Date " + dueDateLabel(d),
				apply: func(v prefixableView) tea.Cmd {
					task := v.getCursorTask()
					project := v.getProject()
					if task == nil || project == nil {
						return nil
					}
					due, _ := dueDateFromDays(d, false)
					cmd := v.singleTaskCoordinator().SetDueDate(taskIdentityFor(task, project), due, false)
					msg := cmd()
					if em, ok := msg.(TransactionErrorMsg); ok {
						return func() tea.Msg { return ErrorMsg{Err: em.Err} }
					}
					return func() tea.Msg { return msg }
				},
			},
		}
	}
}

func recordRepeatClearDueDate() tea.Cmd {
	return func() tea.Msg {
		return RecordRepeatableActionMsg{
			Action: &repeatableAction{
				label: "Clear Due Date",
				apply: func(v prefixableView) tea.Cmd {
					task := v.getCursorTask()
					project := v.getProject()
					if task == nil || project == nil {
						return nil
					}
					if task.DueDate == nil {
						return nil
					}
					cmd := v.singleTaskCoordinator().SetDueDate(taskIdentityFor(task, project), nil, false)
					msg := cmd()
					if em, ok := msg.(TransactionErrorMsg); ok {
						return func() tea.Msg { return ErrorMsg{Err: em.Err} }
					}
					return func() tea.Msg { return msg }
				},
			},
		}
	}
}
