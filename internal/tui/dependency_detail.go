package tui

import (
	"fmt"
	"slices"

	"charm.land/lipgloss/v2"

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

// resolveDependencyTarget scans a loaded population for one locator. The third
// return says whether the locator's project was loaded at all, which is what
// separates a target that is gone from one this view simply cannot read. Both
// the detail rows and the row indicator go through here, so the two surfaces
// never disagree about what a reference resolves to.
func resolveDependencyTarget(reference models.TaskReference, projects []*models.Project) (*models.Project, *models.Task, bool) {
	for _, project := range projects {
		if project == nil || project.ID != reference.ProjectID {
			continue
		}
		return project, project.GetTaskByID(reference.TaskID), true
	}
	return nil, nil, false
}

// lookupDependencyTarget resolves one locator against a loaded population. A
// loaded project whose task is absent reports the same failure as an unloaded
// one, because the detail pane says `unavailable` for both: it can establish
// only that it cannot read the target.
func lookupDependencyTarget(reference models.TaskReference, projects []*models.Project) (dependencyDetail, bool) {
	project, task, _ := resolveDependencyTarget(reference, projects)
	if task == nil {
		return dependencyDetail{}, false
	}
	return dependencyDetailFor(project, task), true
}

// dependencyOutlook is the one aggregate reading of what a task's stored
// references point at. It decides a color and nothing else: no reference is
// added, dropped, or counted differently because of it, and no ordering,
// readiness, or completion rule reads it.
type dependencyOutlook int

const (
	// dependencyOutlookSettled means every reference resolves to a terminal task,
	// or to a task no longer present in a project this view did load.
	dependencyOutlookSettled dependencyOutlook = iota
	// dependencyOutlookUnreadable means nothing is outstanding, but at least one
	// locator points into a project this view did not load. It stays distinct
	// from settled for the same reason dependencyDetailUnavailable is not
	// "missing": calling it settled would assert a fact the view cannot read.
	dependencyOutlookUnreadable
	// dependencyOutlookOutstanding means at least one resolvable target is still
	// nonterminal.
	dependencyOutlookOutstanding
)

// dependencyOutlookFor reads what a task's stored references point at.
// Precedence is deliberate: one outstanding target outranks any number of
// unreadable ones, because a row that has something left to wait on should say
// so rather than hide behind what it could not resolve.
func dependencyOutlookFor(task *models.Task, projects []*models.Project) dependencyOutlook {
	if task == nil {
		return dependencyOutlookSettled
	}
	outlook := dependencyOutlookSettled
	for _, reference := range task.Dependencies {
		var reading dependencyOutlook
		switch _, target, loaded := resolveDependencyTarget(reference, projects); {
		case !loaded:
			reading = dependencyOutlookUnreadable
		case target == nil, target.IsTerminalState():
			reading = dependencyOutlookSettled
		default:
			reading = dependencyOutlookOutstanding
		}
		if reading > outlook {
			outlook = reading
		}
	}
	return outlook
}

// style resolves the outlook to the indicator's foreground.
func (outlook dependencyOutlook) style() lipgloss.Style {
	switch outlook {
	case dependencyOutlookOutstanding:
		return DependencyIndicatorOutstandingStyle
	case dependencyOutlookUnreadable:
		return MutedStyle
	default:
		return DependencyIndicatorSettledStyle
	}
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
