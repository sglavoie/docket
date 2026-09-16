package outliner

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// outlinerFocusContextPrefix labels the Focus breadcrumb on the context row.
// It is named because the readout lists the same breadcrumb under its own
// "focus" label, where repeating the word would be the label said twice.
const outlinerFocusContextPrefix = "Focus: "

// projectFocusContext returns the full single-project Focus path. Width fitting
// belongs to the operational-title renderer so pulse components can yield
// before this active interaction context is shortened.
func (m *OutlinerModel) ProjectFocusContext() string {
	if len(m.Focus.Stack) == 0 || m.Project == nil {
		return ""
	}
	task := m.Project.GetTaskByID(m.Focus.Stack[len(m.Focus.Stack)-1].TaskID)
	if task == nil {
		return ""
	}
	return taskFocusContext(m.Project, task)
}

// taskFocusContext renders the canonical project-qualified hierarchy for the
// operational row. Resolving ParentID through the owning project keeps the
// breadcrumb complete even when a detached projection clone lacks Parent.
func taskFocusContext(project *models.Project, task *models.Task) string {
	if project == nil || task == nil {
		return ""
	}
	projectName := strings.TrimSpace(project.Name)
	if projectName == "" {
		projectName = strings.TrimSpace(project.Slug)
	}
	if projectName == "" {
		projectName = project.ID
	}

	path := []string{task.Title}
	seen := map[string]bool{task.ID: true}
	for parent := focusParent(project, task); parent != nil && !seen[parent.ID]; parent = focusParent(project, parent) {
		seen[parent.ID] = true
		path = append(path, parent.Title)
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return outlinerFocusContextPrefix + "[" + projectName + "] › " + strings.Join(path, " › ")
}

func focusParent(project *models.Project, task *models.Task) *models.Task {
	if task == nil {
		return nil
	}
	if task.Parent != nil {
		return task.Parent
	}
	if task.ParentID != nil {
		return project.GetTaskByID(*task.ParentID)
	}
	return nil
}
