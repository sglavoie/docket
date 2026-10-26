package outliner

import (
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// buildModeCandidateInput completes the shared mode input with the focus-narrowed
// candidates and the membership a mode owns rather than filters for.
func (m *OutlinerModel) buildModeCandidateInput(
	mode chrome.OutlinerModeID,
	projects []*models.Project,
) OutlinerModeInput {
	input := m.buildOutlinerModeInput(projects)
	recent := mode == chrome.ModeRecent
	// Recent reports what happened, so it owns terminal and deferred membership
	// outright. Every other mode admits those populations only when the query
	// says so, and the query is the facets and the sigils together — one rule,
	// read here through the same two accessors every other consumer reads.
	input.IncludeDeferred = recent || m.admitsDeferredPopulation()
	input.IncludeTerminal = recent || m.AdmitsTerminalPopulation()
	input.CandidateTasks = make(map[string][]*models.Task, len(projects))
	input.documentOrder = &documentOrders{}
	for _, project := range projects {
		if project != nil {
			input.CandidateTasks[project.ID] = flattenTaskPointers(m.primaryModeRoots(project))
		}
	}
	if recent {
		input.RecencyKeys = m.freezeRecencyKeys(input.CandidateTasks, projects)
	}
	return input
}

// recentActivityLead returns the newest task Recent admits, along with the
// collapse key of the section holding it. The lead is read from the flat
// section order rather than from the built rows, because grouping later nests a
// promoted child under an older parent and the first row on screen is then not
// the newest task. Returning the section's own key keeps the bucket vocabulary
// in one place.
func (m *OutlinerModel) recentActivityLead() (*models.Task, *models.Project, string, bool) {
	projects := m.ProjectsInCatalogOrder(m.WorkModeScopeProjects())
	// This is the one mode input built outside buildCanonicalModeRows, so the
	// census the hierarchy facet reads would otherwise be whichever scope ran
	// last.
	m.openChildCounts = query.OpenChildCounts(projects)
	input := m.buildModeCandidateInput(chrome.ModeRecent, projects)
	for _, section := range BuildRecentModeSections(input) {
		for _, row := range section.Rows {
			if row.Task == nil || row.Project == nil {
				continue
			}
			if input.TaskFilter != nil && !input.TaskFilter(row.Task, row.Project) {
				continue
			}
			return row.Task, row.Project, section.CollapseKey, true
		}
	}
	return nil, nil, "", false
}

// freezeRecencyKeys returns Recent's ordering timestamps, recording each task's
// UpdatedAt the first time it is seen. The keys are held against the projection
// fingerprint that already invalidates relocation jump history, so a scope,
// filter, search, sort, or fold change re-reads the world while ordinary
// mutations leave the list the user is triaging in place.
func (m *OutlinerModel) freezeRecencyKeys(
	candidates map[string][]*models.Task,
	projects []*models.Project,
) map[taskref.TaskIdentity]time.Time {
	m.Recency.rebase(m.JumpProjectionFingerprint())
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range candidates[project.ID] {
			identity := taskref.For(task, project)
			if !identity.Valid() {
				continue
			}
			m.Recency.freeze(identity, task.UpdatedAt)
		}
	}
	return m.Recency.frozenKeys()
}

func planningNextOrder(runtime *config.RuntimeState) []config.ProjectTaskLocator {
	if runtime == nil {
		return nil
	}
	return runtime.Planning().GetNext()
}

// resolveTodayIDs returns the identity set of daily-planned tasks that
// exist in the scope projects. Derived from the durable plan so it does not
// depend on any externally-maintained indicator cache.
func (m *OutlinerModel) resolveTodayIDs(projects []*models.Project) map[taskref.TaskIdentity]bool {
	ids := make(map[taskref.TaskIdentity]bool)
	if m.Runtime == nil {
		return ids
	}
	for _, item := range m.Runtime.Planning().GetToday() {
		for _, p := range projects {
			if p == nil || p.ID != item.ProjectID {
				continue
			}
			if t := p.GetTaskByID(item.TaskID); t != nil {
				ids[taskref.For(t, p)] = true
			}
			break
		}
	}
	return ids
}

// resolveTodayOrder returns the daily-plan tasks in their durable order,
// scoped to loaded projects.
func (m *OutlinerModel) resolveTodayOrder(projects []*models.Project) []PlanningTask {
	if m.Runtime == nil {
		return nil
	}
	plan := m.Runtime.Planning().GetToday()
	order := make([]PlanningTask, 0, len(plan))
	for _, item := range plan {
		for _, p := range projects {
			if p == nil || p.ID != item.ProjectID {
				continue
			}
			if t := p.GetTaskByID(item.TaskID); t != nil {
				order = append(order, PlanningTask{Task: t, Project: p})
			}
			break
		}
	}
	return order
}

// facetPredicateAt is the one definition of the facet gate: the row half of the
// filter query answered by models.Query.Admits, ANDed with the hierarchy axis,
// evaluated at a single instant. Two consumers read it. The work-mode filter is
// it, and the search projection composes it into the candidate boundary before
// scoring. Free text is deliberately absent: it is one clause answered by one
// matcher in scoreSearchProjectionTask, reached through compileActiveNarrowing,
// so no second definition of "matches" exists to disagree with the tiers.
func (m *OutlinerModel) FacetPredicateAt(now time.Time) func(*models.Task, *models.Project) bool {
	return m.facetPredicateForRowsAt(m.rowPredicateQuery(), now)
}

// facetPredicateForRowsAt also evaluates a diagnostic's detached row query,
// preserving the live hierarchy and dependency restrictions.
//
// The hidden population is decided here too, ahead of the facets, because this
// is the one gate every mode, section, lane, and picker count reads: an
// effectively hidden task is out unless the filter query opens that population.
func (m *OutlinerModel) facetPredicateForRowsAt(query models.Query, now time.Time) func(*models.Task, *models.Project) bool {
	narrows := len(query.Clauses) > 0
	admitsHidden := m.AdmitsHiddenPopulation()
	return func(t *models.Task, project *models.Project) bool {
		if t == nil {
			return false
		}
		if !admitsHidden && models.TaskEffectivelyHidden(m.filterQueryProject(project), t) {
			return false
		}
		if narrows && !query.Admits(t, m.filterQueryProject(project), now) {
			return false
		}
		return m.passesHierarchyFilter(t, project) && m.passesDependencyFilter(t, project)
	}
}

func (m *OutlinerModel) workModeTaskFilterAt(now time.Time) func(*models.Task, *models.Project) bool {
	return m.FacetPredicateAt(now)
}

// passesHierarchyFilter evaluates the hierarchy facet. The parent and leaf axes
// read the scope's open-direct-child census, the same source the ›N marker
// reads, so the facet and the marker agree by construction. The child and root
// axes read the stored ParentID, which survives cloning.
func (m *OutlinerModel) passesHierarchyFilter(task *models.Task, project *models.Project) bool {
	switch m.Filter.Hierarchy {
	case query.HierarchyChild:
		return task.ParentID != nil
	case query.HierarchyRoot:
		return task.ParentID == nil
	case query.HierarchyParent:
		return m.openChildCountFor(task, project) > 0
	case query.HierarchyLeaf:
		return m.openChildCountFor(task, project) == 0
	default:
		return true
	}
}

// passesDependencyFilter evaluates the f~ facet. It sits beside the hierarchy
// axis rather than inside the row predicate for the same reason: blocking is a
// fact about which other tasks name this one, and the reference index that knows
// it is the Outliner's, not the clause evaluator's. Blocked travels with it so
// one facet is answered in one place.
//
// Both readings are one hop and never transitive, and neither is a readiness
// verdict: a soft reference gates nothing, so this narrows what is on screen and
// changes no order, no rank, and no state.
func (m *OutlinerModel) passesDependencyFilter(task *models.Task, project *models.Project) bool {
	if m == nil || m.Filter.Dependency == query.DependencyFilterAll {
		return true
	}
	if task == nil {
		return false
	}
	switch m.Filter.Dependency {
	case query.DependencyFilterBlocked:
		return len(task.Dependencies) > 0
	case query.DependencyFilterBlocking:
		identity := taskref.For(task, m.filterQueryProject(project))
		subject := models.TaskReference{ProjectID: identity.ProjectID, TaskID: identity.TaskID}
		return len(m.dependencyIndex().Incoming(subject)) > 0
	default:
		return true
	}
}

// openChildCountFor resolves a task's census entry, falling back to the scoped
// project the way rowIdentity does. A census miss is 0, which reads as leaf.
func (m *OutlinerModel) openChildCountFor(task *models.Task, project *models.Project) int {
	if project == nil {
		project = m.Project
	}
	return m.openChildCounts[taskref.For(task, project)]
}

// outlinerRowFromModeRow adapts a representation-neutral OutlinerModeRow to a
// concrete flat display row, carrying the owning project ID in cross-project
// scope so badges resolve.
func (m *OutlinerModel) outlinerRowFromModeRow(mr OutlinerModeRow) OutlinerRow {
	row := OutlinerRow{
		Task:               mr.Task,
		Breadcrumb:         mr.Breadcrumb,
		HasVisibleChildren: false,
	}
	if m.Project == nil && mr.Project != nil {
		row.ProjectID = mr.Project.ID
	}
	return row
}

// workModeScopeProjects returns the projects the seven modes range over: the
// range over: the single project when scoped, otherwise the loaded projects
// admitted by the active Context.
func (m *OutlinerModel) WorkModeScopeProjects() []*models.Project {
	if m.Project != nil {
		return []*models.Project{m.Project}
	}
	return m.ScopeProjects()
}

// OutlinerModeRow is the representation-neutral output of a purpose-built row
// strategy. Membership and ordering remain flat here; the presentation layer
// later nests relatives that are both explicit members of the same section. A
// promoted child keeps its ancestor breadcrumb, and unselected descendants are
// never added implicitly.
type OutlinerModeRow struct {
	Task       *models.Task
	Project    *models.Project
	Breadcrumb string
}

// OutlinerModeInput bundles the scoped data a canonical row strategy needs.
// CandidateTasks optionally narrows eligibility to the current focus projection;
// a present project entry, including an empty one, is authoritative.
type OutlinerModeInput struct {
	Projects []*models.Project
	TodayIDs map[taskref.TaskIdentity]bool
	// CandidateTasks optionally narrows the tasks eligible for a projection per
	// project. A missing project entry falls back to Project.Tasks.
	CandidateTasks map[string][]*models.Task
	// TodayOrder and NextOrder are the durable project-qualified planning orders.
	TodayOrder  []PlanningTask
	NextOrder   []config.ProjectTaskLocator
	TaskFilter  func(task *models.Task, project *models.Project) bool
	TaskExclude func(task *models.Task, project *models.Project) bool
	// RecencyKeys freezes each task's Recent ordering timestamp so triaging a row
	// does not move it out from under the cursor. A task the map does not name
	// orders by its live UpdatedAt.
	RecencyKeys     map[taskref.TaskIdentity]time.Time
	Now             time.Time
	IncludeDeferred bool
	IncludeTerminal bool
	// documentOrder is shared by every copy of one built input; see
	// documentOrders.
	documentOrder *documentOrders
}

func (in OutlinerModeInput) now() time.Time {
	if in.Now.IsZero() {
		return time.Now()
	}
	return in.Now
}

func (in OutlinerModeInput) recencyKey(task *models.Task, project *models.Project) time.Time {
	if frozen, ok := in.RecencyKeys[taskref.For(task, project)]; ok {
		return frozen
	}
	return task.UpdatedAt
}

// BuildTodayModeRows realizes the explicit ordered Today projection. Every row
// is a durable member; section-local hierarchy is added by the row appender.
func BuildTodayModeRows(in OutlinerModeInput) []OutlinerModeRow {
	projects := modeProjectIndex(in.Projects)
	now := in.now()
	rows := make([]OutlinerModeRow, 0, len(in.TodayOrder))
	seen := make(map[taskref.TaskIdentity]bool, len(in.TodayOrder))
	for _, item := range in.TodayOrder {
		if item.Task == nil || item.Project == nil {
			continue
		}
		project := projects[item.Project.ID]
		if project == nil {
			continue
		}
		task := project.GetTaskByID(item.Task.ID)
		identity := taskref.TaskIdentity{ProjectID: project.ID, TaskID: item.Task.ID}
		if task == nil || seen[identity] || !modeCandidateContains(in, identity) {
			continue
		}
		seen[identity] = true
		if !planningTodayEligible(task) || (task.IsDeferredAt(now) && !in.IncludeDeferred) {
			continue
		}
		if in.TaskExclude != nil && in.TaskExclude(task, project) {
			continue
		}
		if in.TaskFilter != nil && !in.TaskFilter(task, project) {
			continue
		}
		rows = append(rows, planningModeRow(task, project))
	}
	return rows
}

// BuildNextModeRows realizes the explicit ordered Next projection. Every row is
// a durable member; section-local hierarchy is added by the row appender.
func BuildNextModeRows(in OutlinerModeInput) []OutlinerModeRow {
	projects := modeProjectIndex(in.Projects)
	now := in.now()
	rows := make([]OutlinerModeRow, 0, len(in.NextOrder))
	seen := make(map[taskref.TaskIdentity]bool, len(in.NextOrder))
	for _, locator := range in.NextOrder {
		identity := taskref.TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}
		if seen[identity] || !modeCandidateContains(in, identity) {
			continue
		}
		seen[identity] = true
		project := projects[locator.ProjectID]
		if project == nil {
			continue
		}
		task := project.GetTaskByID(locator.TaskID)
		if task == nil || !task.State().IsValid() || task.IsTerminalState() ||
			(task.IsDeferredAt(now) && !in.IncludeDeferred) {
			continue
		}
		if in.TaskExclude != nil && in.TaskExclude(task, project) {
			continue
		}
		if in.TaskFilter != nil && !in.TaskFilter(task, project) {
			continue
		}
		rows = append(rows, planningModeRow(task, project))
	}
	return rows
}

func modeProjectIndex(projects []*models.Project) map[string]*models.Project {
	index := make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project != nil {
			index[project.ID] = project
		}
	}
	return index
}

func modeCandidateContains(in OutlinerModeInput, identity taskref.TaskIdentity) bool {
	candidates, constrained := in.CandidateTasks[identity.ProjectID]
	if !constrained {
		return true
	}
	for _, task := range candidates {
		if task != nil && task.ID == identity.TaskID {
			return true
		}
	}
	return false
}

func planningModeRow(task *models.Task, project *models.Project) OutlinerModeRow {
	return OutlinerModeRow{
		Task:       task,
		Project:    project,
		Breadcrumb: taskAncestorBreadcrumb(task),
	}
}
