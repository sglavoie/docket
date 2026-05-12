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
					if task == nil {
						return nil
					}
					project := v.getProject()
					v.setUndoContext("Set Priority", fmt.Sprintf("P%d: %s", p, task.Title))
					result := applyPriorityChange(v.workflowPersistFn(), project, task.ID, p)
					if result.Err != nil {
						return func() tea.Msg { return ErrorMsg{Err: result.Err} }
					}
					if result.StatusMessage != "" {
						v.setStatusMessage(result.StatusMessage)
					}
					v.setMutationFlash(priorityMutationFlashBg(p))
					v.refreshView()
					return tea.Batch(result.Cmd, mutationFlashTimeout())
				},
			},
		}
	}
}

func recordRepeatStage(stage string) tea.Cmd {
	s := stage
	label := s
	if label == "" {
		label = "(cleared)"
	}
	return func() tea.Msg {
		return RecordRepeatableActionMsg{
			Action: &repeatableAction{
				label: "Set Stage " + label,
				apply: func(v prefixableView) tea.Cmd {
					task := v.getCursorTask()
					if task == nil {
						return nil
					}
					project := v.getProject()
					v.setUndoContext("Set Stage", fmt.Sprintf("%s: %s", label, task.Title))
					result := applyStageChange(v.workflowPersistFn(), project, task.ID, s)
					if result.Err != nil {
						return func() tea.Msg { return ErrorMsg{Err: result.Err} }
					}
					if result.StatusMessage != "" {
						v.setStatusMessage(result.StatusMessage)
					}
					v.refreshView()
					return result.Cmd
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
					if task == nil {
						return nil
					}
					project := v.getProject()
					v.setUndoContext("Set Due Date", fmt.Sprintf("%s: %s", dueDateLabel(d), task.Title))
					result := applyQuickDueDate(v.workflowPersistFn(), project, task.ID, d)
					if result.Err != nil {
						return func() tea.Msg { return ErrorMsg{Err: result.Err} }
					}
					if result.StatusMessage != "" {
						v.setStatusMessage(result.StatusMessage)
					}
					v.refreshView()
					return result.Cmd
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
					if task == nil {
						return nil
					}
					project := v.getProject()
					v.setUndoContext("Set Due Date", "cleared: "+task.Title)
					result := applyClearDueDate(v.workflowPersistFn(), project, task.ID)
					if result.Err != nil {
						return func() tea.Msg { return ErrorMsg{Err: result.Err} }
					}
					if result.StatusMessage != "" {
						v.setStatusMessage(result.StatusMessage)
					}
					v.refreshView()
					return result.Cmd
				},
			},
		}
	}
}
