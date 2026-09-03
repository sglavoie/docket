package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
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

// lookupDependencyTarget resolves one locator through the index. A loaded
// project whose task is absent reports the same failure as an unloaded one,
// because the detail pane says `unavailable` for both: it can establish only
// that it cannot read the target.
func lookupDependencyTarget(
	reference models.TaskReference,
	index *app.DependencyIndex,
) (dependencyDetail, bool) {
	project, task, _ := index.Lookup(reference)
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
	// dependencyOutlookUnreadable means nothing is blocked, but at least one
	// locator points into a project this view did not load. It stays distinct
	// from settled for the same reason dependencyDetailUnavailable is not
	// "missing": calling it settled would assert a fact the view cannot read.
	dependencyOutlookUnreadable
	// dependencyOutlookBlocked means at least one resolvable target is still
	// nonterminal.
	dependencyOutlookBlocked
)

// dependencyCensus is one task's outgoing references reduced to the numbers
// every surface shows. The row paints `~S/N` from it and Peek's Dependencies
// line repeats the same pair, so the two can never report different arithmetic
// over the same population.
//
// total is the stored-locator count and nothing else: a reference is counted
// whatever its target's state or availability, which is what keeps the number a
// fact about the task rather than a fact about what the view could read.
type dependencyCensus struct {
	total      int
	settled    int
	blocked    int
	unreadable int
}

// dependencyCensusFor reads what a task's stored references point at, in one
// pass over the index.
func dependencyCensusFor(task *models.Task, index *app.DependencyIndex) dependencyCensus {
	census := dependencyCensus{}
	if task == nil {
		return census
	}
	census.total = len(task.Dependencies)
	for _, reference := range task.Dependencies {
		switch index.Status(reference) {
		case app.DependencyTargetUnavailable:
			census.unreadable++
		case app.DependencyTargetOpen:
			census.blocked++
		default:
			// Terminal and missing are both settled: a target this view did load
			// and can no longer find has nothing left to wait on either.
			census.settled++
		}
	}
	return census
}

// outlook resolves the census to its color. Precedence is deliberate: one
// blocked target outranks any number of unreadable ones, because a row that has
// something left to wait on should say so rather than hide behind what it could
// not resolve.
func (c dependencyCensus) outlook() dependencyOutlook {
	switch {
	case c.blocked > 0:
		return dependencyOutlookBlocked
	case c.unreadable > 0:
		return dependencyOutlookUnreadable
	default:
		return dependencyOutlookSettled
	}
}

// counts renders the pair both surfaces show. A wholly settled set is written as
// the bare total rather than `N/N`, because a fraction whose halves agree says
// only that nothing is left, and the shorter form says it in fewer columns.
func (c dependencyCensus) counts() string {
	if c.settled == c.total {
		return fmt.Sprintf("%d", c.total)
	}
	return fmt.Sprintf("%d/%d", c.settled, c.total)
}

// style resolves the outlook to the indicator's foreground.
func (outlook dependencyOutlook) style() lipgloss.Style {
	switch outlook {
	case dependencyOutlookBlocked:
		return theme.DependencyIndicatorBlockedStyle
	case dependencyOutlookUnreadable:
		return theme.MutedStyle
	default:
		return theme.DependencyIndicatorSettledStyle
	}
}

// outgoingDependencyDetails describes every stored reference in stored order.
// Every reference produces an entry: the list is what the task holds, so an
// entry is never dropped because its target could not be read, and the order is
// the one `-~task` and `~0` round-trip against.
func outgoingDependencyDetails(task *models.Task, index *app.DependencyIndex) []dependencyDetail {
	if task == nil || len(task.Dependencies) == 0 {
		return nil
	}
	details := make([]dependencyDetail, 0, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		if detail, ok := lookupDependencyTarget(reference, index); ok {
			details = append(details, detail)
			continue
		}
		details = append(details, dependencyDetailUnresolved(reference))
	}
	return details
}

// incomingDependencyDetails lists the tasks naming this one, reading only the
// population the index was built over. Dependents are never stored, so this list
// is the whole of what the product knows about them and it is honestly partial:
// a task in an archived or unloaded project may name the subject without
// appearing here, and no archived project is opened merely to decorate a detail
// pane.
func incomingDependencyDetails(identity TaskIdentity, index *app.DependencyIndex) []dependencyDetail {
	if identity.ProjectID == "" || identity.TaskID == "" {
		return nil
	}
	subject := models.TaskReference{ProjectID: identity.ProjectID, TaskID: identity.TaskID}
	incoming := index.Incoming(subject)
	if len(incoming) == 0 {
		return nil
	}
	details := make([]dependencyDetail, 0, len(incoming))
	for _, reference := range incoming {
		if detail, ok := lookupDependencyTarget(reference, index); ok {
			details = append(details, detail)
		}
	}
	return details
}

// dependencyDetailProjects is the population the index is built over. It is the
// loaded set rather than the scoped one: a project scope narrows what the
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

// dependencyIndex is the one reading of the reference graph a render pass uses.
// It is built lazily over dependencyDetailProjects and held until that
// population is replaced, which is what turns the old per-reference-per-row
// linear scan into a pair of map lookups.
//
// Invalidation is by replacement, never by patching: invalidateDependencyIndex
// drops it wherever the loaded population changes or a mutation sets the
// cross-project force flag, and the next reader rebuilds it.
func (m *OutlinerModel) dependencyIndex() *app.DependencyIndex {
	if m == nil {
		return nil
	}
	if m.dependencyIndexCache == nil {
		m.dependencyIndexCache = app.NewDependencyIndex(m.dependencyDetailProjects())
	}
	return m.dependencyIndexCache
}

// invalidateDependencyIndex drops the cached reading.
func (m *OutlinerModel) invalidateDependencyIndex() {
	if m == nil {
		return
	}
	m.dependencyIndexCache = nil
}

// setLoadedProjects installs a new loaded population. It exists so replacing
// that population and dropping the reading derived from it are one act: a
// caller that assigned the slice directly would leave an index describing
// projects the Outliner no longer holds until the next rebuild pass.
func (m *OutlinerModel) setLoadedProjects(projects []*models.Project) {
	if m == nil {
		return
	}
	m.loadedProjects = projects
	m.invalidateDependencyIndex()
}
