package outliner

import (
	"slices"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// departurePlacement is where a task stood, along the axes a departure can
// move it on: its project, its plan, its status, and whether it is hidden.
// The zero value is a task the view could not resolve.
type departurePlacement struct {
	resolved  bool
	projectID string
	plan      string
	state     models.WorkflowState
	hidden    bool
}

// departurePlacement reads where the task stands now. It looks the task up
// by its ID in every loaded project, not only by its identity, because a
// move to another project keeps the task ID and changes the project half.
func (m *OutlinerModel) departurePlacement(identity taskref.TaskIdentity) departurePlacement {
	task, project := m.TaskAndProjectForIdentity(identity)
	if task == nil {
		task, project = m.taskByIDAnywhere(identity.TaskID)
	}
	if task == nil || project == nil {
		return departurePlacement{}
	}
	return departurePlacement{
		resolved:  true,
		projectID: project.ID,
		plan:      m.planningMembership(project.ID, task.ID),
		state:     task.State(),
		hidden:    models.TaskEffectivelyHidden(project, task),
	}
}

// taskByIDAnywhere finds a task by its ID among the loaded projects.
func (m *OutlinerModel) taskByIDAnywhere(taskID string) (*models.Task, *models.Project) {
	if taskID == "" {
		return nil, nil
	}
	if m.Project != nil {
		if task := m.Project.GetTaskByID(taskID); task != nil {
			return task, m.Project
		}
	}
	for _, project := range m.Population.Projects {
		if project == nil {
			continue
		}
		if task := project.GetTaskByID(taskID); task != nil {
			return task, project
		}
	}
	return nil, nil
}

// planningMembership is the plan token a task belongs to, "today", "next", or
// nothing.
func (m *OutlinerModel) planningMembership(projectID, taskID string) string {
	if m.Runtime == nil {
		return ""
	}
	locator := config.ProjectTaskLocator{ProjectID: projectID, TaskID: taskID}
	planning := m.Runtime.Planning()
	switch {
	case slices.Contains(planning.GetToday(), locator):
		return "today"
	case slices.Contains(planning.GetNext(), locator):
		return "next"
	}
	return ""
}

// departureDestination names where a departed task went, for the ghost to
// show in place of a strike, and the ink to show it in. It compares where the
// task stood at the keypress with where it stands after the commit and names
// the one change a reader asked for, most specific first: a project, then a
// plan it joined, then a status, then hiding. A cancel, a delete, a task the
// view can no longer find, and a departure no axis explains name nothing, so
// they keep the strike.
func (m *OutlinerModel) departureDestination(capture *OutlinerRelocationCapture) (string, lipgloss.Style) {
	before := capture.departureFrom
	if !before.resolved {
		return "", lipgloss.Style{}
	}
	after := m.departurePlacement(capture.Task)
	if !after.resolved || after.state == models.StateCanceled || after.state == models.StateDone {
		return "", lipgloss.Style{}
	}
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	if after.projectID != before.projectID {
		project := m.crossResolveProject(after.projectID)
		if project == nil && m.Project != nil && m.Project.ID == after.projectID {
			project = m.Project
		}
		if project != nil && project.Name != "" {
			return project.Name, theme.ProjectColorStyle(project)
		}
	}
	if after.plan != "" && after.plan != before.plan {
		return chrome.PlanStatusValue(after.plan), accent
	}
	if after.state != before.state || after.plan != before.plan {
		return models.WorkflowStateGroupForState(after.state).Label(), accent
	}
	if after.hidden && !before.hidden {
		return "Hidden", theme.MutedStyle
	}
	return "", lipgloss.Style{}
}
