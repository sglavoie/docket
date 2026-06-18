package tui

import (
	"fmt"
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

func taskPropertyChangedWithUndo(task *models.Task, action string) TaskPropertyChangedMsg {
	msg := TaskPropertyChangedMsg{
		Task:        task,
		NeedsSave:   true,
		UndoAction:  action,
		UndoContext: taskUndoContext(task),
	}
	if task != nil {
		msg.UndoTaskIDs = []string{task.ID}
	}
	return msg
}

func bulkTaskPropertyChangedWithUndo(action, context string, taskIDs []string) TaskPropertyChangedMsg {
	return TaskPropertyChangedMsg{
		NeedsSave:   true,
		UndoAction:  action,
		UndoContext: context,
		UndoTaskIDs: taskIDs,
	}
}

func (m *AppModel) taskMutationSaveOptions(task *models.Task, action string) ProjectSaveOptions {
	opts := ProjectSaveOptions{
		UndoAction:  action,
		UndoContext: taskUndoContext(task),
	}
	if task != nil {
		opts.UndoTaskIDs = []string{task.ID}
	}
	if m.config != nil && m.config.UndoHistoryDepth != nil {
		opts.UndoDepth = *m.config.UndoHistoryDepth
	}
	return opts
}

func taskUndoContext(task *models.Task) string {
	if task == nil {
		return ""
	}
	if task.ShortID > 0 {
		return fmt.Sprintf("#%d %s", task.ShortID, task.Title)
	}
	return task.Title
}

func sortedSelectedTaskIDs(selected map[string]bool) []string {
	ids := make([]string, 0, len(selected))
	for id, ok := range selected {
		if ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
