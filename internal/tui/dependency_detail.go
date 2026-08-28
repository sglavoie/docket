package tui

import (
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/models"
)

// dependencyDetailUnavailable is the status a reference wears when this
// workspace view did not load the project it points into. It is deliberately
// not "missing": the target may well exist in an archived or unloaded project,
// and a detail pane that said otherwise would report a fact it cannot know.
const dependencyDetailUnavailable = "unavailable"

// dependencyDetail is one read-only row of the Dependencies or Dependents
// detail. Both directions render through this single shape, so an outgoing
// reference and an incoming one are described in the same vocabulary and a
// reader never has to work out which list they are looking at.
//
// A loaded entry names its project, short ID, title, and state. An unloaded one
// keeps its stable locator and says only that it is unavailable, which preserves
// the durable value rather than dropping the entry.
type dependencyDetail struct {
	label string
	state string
}

// String renders one entry as `Project #7 Fix login (doing)`.
func (d dependencyDetail) String() string {
	if d.state == "" {
		return d.label
	}
	return d.label + " (" + d.state + ")"
}

// dependencyDetailFor describes one loaded task.
func dependencyDetailFor(project *models.Project, task *models.Task) dependencyDetail {
	label := fmt.Sprintf("#%d %s", task.ShortID, task.Title)
	if project != nil && project.Name != "" {
		label = project.Name + " " + label
	}
	return dependencyDetail{label: label, state: string(task.State())}
}

// dependencyDetailUnresolved describes a reference whose project this view did
// not load. The stable locator is the whole value: it is what storage holds and
// what the edit grammar round-trips, so showing it keeps the entry actionable
// even though nothing about the target can be read.
func dependencyDetailUnresolved(reference models.TaskReference) dependencyDetail {
	return dependencyDetail{label: reference.Locator(), state: dependencyDetailUnavailable}
}

// lookupDependencyTarget resolves one locator against a loaded population.
func lookupDependencyTarget(reference models.TaskReference, projects []*models.Project) (dependencyDetail, bool) {
	for _, project := range projects {
		if project == nil || project.ID != reference.ProjectID {
			continue
		}
		task := project.GetTaskByID(reference.TaskID)
		if task == nil {
			return dependencyDetail{}, false
		}
		return dependencyDetailFor(project, task), true
	}
	return dependencyDetail{}, false
}

// outgoingDependencyDetails describes every stored reference in stored order.
// Every reference produces an entry: the list is what the task holds, so an
// entry is never dropped because its target could not be read.
func outgoingDependencyDetails(task *models.Task, projects []*models.Project) []dependencyDetail {
	if task == nil || len(task.Dependencies) == 0 {
		return nil
	}
	details := make([]dependencyDetail, 0, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		if detail, ok := lookupDependencyTarget(reference, projects); ok {
			details = append(details, detail)
			continue
		}
		details = append(details, dependencyDetailUnresolved(reference))
	}
	return details
}

// incomingDependencyDetails computes the tasks naming this one, walking only the
// projects this view loaded. Dependents are never stored, so this list is the
// whole of what the product knows about them and it is honestly partial: a task
// in an archived or unloaded project may name the subject without appearing
// here, and no archived project is opened merely to decorate a detail pane.
func incomingDependencyDetails(identity TaskIdentity, projects []*models.Project) []dependencyDetail {
	if identity.ProjectID == "" || identity.TaskID == "" {
		return nil
	}
	subject := models.TaskReference{ProjectID: identity.ProjectID, TaskID: identity.TaskID}
	var details []dependencyDetail
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || len(task.Dependencies) == 0 {
				continue
			}
			if !slices.Contains(task.Dependencies, subject) {
				continue
			}
			details = append(details, dependencyDetailFor(project, task))
		}
	}
	return details
}

// dependencyDetailProjects is the population both detail directions read. It is
// the loaded set rather than the scoped one: a project scope narrows what the
// Outliner shows, not what it has in memory, and a reference into a loaded but
// unscoped project is knowable.
func (m *OutlinerModel) dependencyDetailProjects() []*models.Project {
	if m == nil {
		return nil
	}
	if len(m.loadedProjects) > 0 {
		return m.loadedProjects
	}
	if m.project != nil {
		return []*models.Project{m.project}
	}
	return nil
}
