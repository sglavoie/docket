package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// buildReparentValidParents returns tasks that are valid parents for reparenting task.
// Excludes the task itself, its descendants, and tasks that can't parent the task's type.
func buildReparentValidParents(project *models.Project, task *models.Task) []*models.Task {
	if project == nil || task == nil {
		return nil
	}
	var parents []*models.Task
	excluded := taskDescendantIDs(task)
	for _, t := range project.Tasks {
		if t.ID == task.ID || excluded[t.ID] {
			continue
		}
		if t.CanAddChild(task.Type) {
			parents = append(parents, t)
		}
	}
	return parents
}

// taskDescendantIDs returns the set of IDs of all descendants of task.
func taskDescendantIDs(task *models.Task) map[string]bool {
	ids := make(map[string]bool)
	for _, child := range task.Children {
		ids[child.ID] = true
		for id := range taskDescendantIDs(child) {
			ids[id] = true
		}
	}
	return ids
}

// resolveTaskAndProject returns the task under the cursor and its owning
// project. In single-project mode, project comes from the view's getProject().
// In cross-project mode (project == nil), it falls back to getTaskRef() which
// resolves the project per-row.
func resolveTaskAndProject(v primaryProjectView, project *models.Project) (*models.Task, *models.Project) {
	task := v.getCursorTask()
	if task == nil {
		return nil, nil
	}
	if project != nil {
		return task, project
	}
	// Cross-project mode: resolve project from the row
	type taskRefProvider interface {
		getTaskRef() (*models.Task, *models.Project)
	}
	if trp, ok := v.(taskRefProvider); ok {
		return trp.getTaskRef()
	}
	return nil, nil
}

// handleResolvedTaskKey handles keys that need resolveTaskAndProject before acting.
func handleResolvedTaskKey(key string, v primaryProjectView, project *models.Project) (tea.Cmd, bool) {
	task, taskProject := resolveTaskAndProject(v, project)
	if task == nil || taskProject == nil {
		return nil, true
	}
	switch key {
	case "S":
		return func() tea.Msg {
			return ShowScoreEditorMsg{Task: task, Project: taskProject}
		}, true
	case "ctrl+.":
		return func() tea.Msg {
			return OpenQuickNoteMsg{Task: task, Project: taskProject}
		}, true
	case "ctrl+m":
		validParents := buildReparentValidParents(taskProject, task)
		taskID := task.ID
		var currentParentID *string
		if task.ParentID != nil {
			id := *task.ParentID
			currentParentID = &id
		}
		return func() tea.Msg {
			return OpenParentSelectorMsg{
				ValidParents:   validParents,
				CurrentID:      currentParentID,
				AllowNone:      true,
				Source:         ViewProjectTree,
				ReparentTaskID: taskID,
			}
		}, true
	}
	return nil, false
}
